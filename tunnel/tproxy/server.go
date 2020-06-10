// +build linux

package tproxy

import (
	"context"
	"github.com/LiamHaworth/go-tproxy"
	"github.com/p4gefau1t/trojan-go/common"
	"github.com/p4gefau1t/trojan-go/config"
	"github.com/p4gefau1t/trojan-go/log"
	"github.com/p4gefau1t/trojan-go/tunnel"
	"github.com/p4gefau1t/trojan-go/tunnel/dokodemo"
	"io"
	"net"
	"sync"
	"time"
)

const MaxPacketSize = 1024 * 8

type Server struct {
	tcpListener net.Listener
	udpListener *net.UDPConn
	packetChan  chan tunnel.PacketConn
	timeout     time.Duration
	mappingLock sync.Mutex
	mapping     map[string]*PacketConn
	ctx         context.Context
	cancel      context.CancelFunc
}

func (s *Server) Close() error {
	s.cancel()
	s.tcpListener.Close()
	return s.udpListener.Close()
}

func (s *Server) AcceptConn(tunnel.Tunnel) (tunnel.Conn, error) {
	conn, err := s.tcpListener.Accept()
	if err != nil {
		return nil, common.NewError("tproxy failed to accept connection").Base(err)
	}
	addr, err := getOriginalTCPDest(conn.(*tproxy.Conn).TCPConn)
	if err != nil {
		return nil, common.NewError("tproxy failed to obtain original address of tcp socket").Base(err)
	}
	address, err := tunnel.NewAddressFromAddr("tcp", addr.String())
	common.Must(err)
	return &Conn{
		metadata: &tunnel.Metadata{
			Address: address,
		},
		Conn: conn,
	}, nil
}

func (s *Server) packetDispatchLoop() {
	for {
		buf := make([]byte, MaxPacketSize)
		n, src, dst, err := tproxy.ReadFromUDP(s.udpListener, buf)
		if err != nil {
			s.cancel()
			log.Error("tproxy failed to read from udp")
			return
		}
		log.Debug("udp packet from", src, "to", dst)
		s.mappingLock.Lock()
		if conn, found := s.mapping[src.String()]; found {
			conn.Input <- buf[:n]
			s.mappingLock.Unlock()
			continue
		}
		log.Info("tproxy udp session, from", src, "to", dst)
		ctx, cancel := context.WithCancel(s.ctx)
		conn := &PacketConn{
			dokodemo.PacketConn{
				Input:      make(chan []byte, 16),
				Output:     make(chan []byte, 16),
				Source:     src,
				PacketConn: s.udpListener,
				Context:    ctx,
				CancelFunc: cancel,
				M: &tunnel.Metadata{
					Address: &tunnel.Address{},
				},
			},
		}
		s.mapping[src.String()] = conn
		s.mappingLock.Unlock()

		conn.Input <- buf[:n]
		s.packetChan <- conn

		go func(conn *PacketConn) {
			for {
				select {
				case payload := <-conn.Output:
					_, err := s.udpListener.WriteTo(payload, conn.Source)
					if err != nil {
						log.Error(common.NewError("tproxy udp write error").Base(err))
						return
					}
				case <-s.ctx.Done():
					return
				case <-time.After(s.timeout):
					s.mappingLock.Lock()
					delete(s.mapping, conn.Source.String())
					s.mappingLock.Unlock()
					conn.Close()
					log.Debug("timeout packet session closed", conn.Source.String())
					return
				}
			}
		}(conn)
	}
}

func (s *Server) AcceptPacket(tunnel.Tunnel) (tunnel.PacketConn, error) {
	select {
	case conn := <-s.packetChan:
		return conn, nil
	case <-s.ctx.Done():
		return nil, io.EOF
	}
}

func NewServer(ctx context.Context, _ tunnel.Server) (*Server, error) {
	cfg := config.FromContext(ctx, Name).(*Config)
	ctx, cancel := context.WithCancel(ctx)
	listenAddr := tunnel.NewAddressFromHostPort("tcp", cfg.LocalHost, cfg.LocalPort)
	ip, err := listenAddr.ResolveIP()
	if err != nil {
		return nil, common.NewError("invalid tproxy local address").Base(err)
	}
	tcpListener, err := tproxy.ListenTCP("tcp", &net.TCPAddr{
		IP:   ip,
		Port: cfg.LocalPort,
	})
	if err != nil {
		return nil, common.NewError("tproxy failed to listen tcp").Base(err)
	}

	udpListener, err := tproxy.ListenUDP("udp", &net.UDPAddr{
		IP:   ip,
		Port: cfg.LocalPort,
	})
	if err != nil {
		return nil, common.NewError("tproxy failed to listen udp").Base(err)
	}

	server := &Server{
		tcpListener: tcpListener,
		udpListener: udpListener,
		ctx:         ctx,
		cancel:      cancel,
		timeout:     time.Duration(cfg.UDPTimeout) * time.Second,
		mapping:     make(map[string]*PacketConn),
		packetChan:  make(chan tunnel.PacketConn),
	}
	go server.packetDispatchLoop()
	return server, nil
}
