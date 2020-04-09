package trojan

import (
	"bufio"
	"crypto/tls"
	"io"

	"github.com/p4gefau1t/trojan-go/common"
	"github.com/p4gefau1t/trojan-go/conf"
	"github.com/p4gefau1t/trojan-go/log"
	"github.com/p4gefau1t/trojan-go/protocol"
)

type TrojanOutboundConnSession struct {
	protocol.ConnSession
	config        *conf.GlobalConfig
	conn          io.ReadWriteCloser
	bufReadWriter *bufio.ReadWriter
	request       *protocol.Request
	sent          int
	recv          int
}

func (o *TrojanOutboundConnSession) Write(p []byte) (int, error) {
	n, err := o.bufReadWriter.Write(p)
	o.bufReadWriter.Flush()
	o.sent += n
	return n, err
}

func (o *TrojanOutboundConnSession) Read(p []byte) (int, error) {
	n, err := o.bufReadWriter.Read(p)
	o.recv += n
	return n, err
}

func (o *TrojanOutboundConnSession) Close() error {
	log.Info("conn to", o.request, "closed", "sent:", common.HumanFriendlyTraffic(o.sent), "recv:", common.HumanFriendlyTraffic(o.recv))
	return o.conn.Close()
}

func (o *TrojanOutboundConnSession) writeRequest() error {
	hash := ""
	for k := range o.config.Hash {
		hash = k
		break
	}
	crlf := []byte("\r\n")
	o.bufReadWriter.Write([]byte(hash))
	o.bufReadWriter.Write(crlf)
	o.bufReadWriter.WriteByte(byte(o.request.Command))
	err := protocol.WriteAddress(o.bufReadWriter, o.request)
	if err != nil {
		return common.NewError("failed to write address").Base(err)
	}
	o.bufReadWriter.Write(crlf)
	return o.bufReadWriter.Flush()
}

func NewOutboundConnSession(req *protocol.Request, conn io.ReadWriteCloser, config *conf.GlobalConfig) (protocol.ConnSession, error) {
	if conn == nil {
		tlsConfig := &tls.Config{
			CipherSuites:           config.TLS.CipherSuites,
			RootCAs:                config.TLS.CertPool,
			ServerName:             config.TLS.SNI,
			InsecureSkipVerify:     !config.TLS.Verify,
			SessionTicketsDisabled: !config.TLS.SessionTicket,
			ClientSessionCache:     tls.NewLRUClientSessionCache(-1),
		}
		tlsConn, err := tls.Dial("tcp", config.RemoteAddr.String(), tlsConfig)
		if err != nil {
			return nil, common.NewError("cannot dial to the remote server").Base(err)
		}
		if config.LogLevel == 0 {
			state := tlsConn.ConnectionState()
			chain := state.VerifiedChains
			log.Debug("TLS handshaked", "cipher:", tls.CipherSuiteName(state.CipherSuite), "resume:", state.DidResume)
			for i := range chain {
				for j := range chain[i] {
					log.Debug("subject:", chain[i][j].Subject, ", issuer:", chain[i][j].Issuer)
				}
			}
		}
		conn = tlsConn
		if config.Websocket.Enabled {
			ws, err := NewOutboundWebosocket(tlsConn, config)
			if err != nil {
				return nil, common.NewError("failed to start websocket connection").Base(err)
			}
			conn = ws
		}
	}
	o := &TrojanOutboundConnSession{
		request:       req,
		config:        config,
		conn:          conn,
		bufReadWriter: common.NewBufReadWriter(conn),
	}
	if err := o.writeRequest(); err != nil {
		return nil, common.NewError("failed to write request").Base(err)
	}
	return o, nil
}
