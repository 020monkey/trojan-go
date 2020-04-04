package conf

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"strings"

	"github.com/p4gefau1t/trojan-go/common"
	"github.com/p4gefau1t/trojan-go/log"
)

func convertToAddr(preferV4 bool, host string, port int) (*net.TCPAddr, error) {
	ip := net.ParseIP(host)
	if ip != nil {
		return &net.TCPAddr{
			IP:   ip,
			Port: port,
		}, nil
	}
	if preferV4 {
		return net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", host, port))
	}
	return net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", host, port))
}

func ParseJSON(data []byte) (*GlobalConfig, error) {
	var config GlobalConfig

	//default settings
	config.TLS.Verify = true
	config.TLS.VerifyHostname = true
	config.TLS.SessionTicket = true
	config.Mux.IdleTimeout = 60
	config.Mux.Concurrency = 8
	config.MySQL.CheckRate = 60
	config.Router.DefaultPolicy = "proxy"

	err := json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	log.DefaultLogger.SetLogLevel(log.LogLevel(config.LogLevel))

	config.Hash = make(map[string]string)
	for _, password := range config.Passwords {
		config.Hash[common.SHA224String(password)] = password
	}
	switch config.RunType {
	case Client, NAT:
		if len(config.Passwords) == 0 {
			return nil, common.NewError("no password found")
		}
		if config.TLS.CertPath == "" {
			log.DefaultLogger.Warn("cert of the remote server is not specified. using default CA list.")
			break
		}
		serverCertBytes, err := ioutil.ReadFile(config.TLS.CertPath)
		if err != nil {
			return nil, common.NewError("failed to load cert file").Base(err)
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(serverCertBytes)
		config.TLS.CertPool = pool
	case Server:
		if len(config.Passwords) == 0 {
			return nil, common.NewError("no password found")
		}
		if config.TLS.KeyPassword != "" {
			keyFile, err := ioutil.ReadFile(config.TLS.KeyPath)
			if err != nil {
				return nil, common.NewError("failed to load key file").Base(err)
			}
			keyBlock, _ := pem.Decode(keyFile)
			if keyBlock == nil {
				return nil, common.NewError("failed to decode key file").Base(err)
			}
			decryptedKey, err := x509.DecryptPEMBlock(keyBlock, []byte(config.TLS.KeyPassword))
			if err == nil {
				return nil, common.NewError("failed to decrypt key").Base(err)
			}

			certFile, err := ioutil.ReadFile(config.TLS.CertPath)
			certBlock, _ := pem.Decode(certFile)
			if certBlock == nil {
				return nil, common.NewError("failed to decode cert file").Base(err)
			}

			keyPair, err := tls.X509KeyPair(certBlock.Bytes, decryptedKey)
			if err != nil {
				return nil, err
			}

			config.TLS.KeyPair = []tls.Certificate{keyPair}
		} else {
			keyPair, err := tls.LoadX509KeyPair(config.TLS.CertPath, config.TLS.KeyPath)
			if err != nil {
				return nil, common.NewError("failed to load key pair").Base(err)
			}
			config.TLS.KeyPair = []tls.Certificate{keyPair}
		}
	case Forward:
	default:
		return nil, common.NewError("invalid run type")
	}

	localAddr, err := convertToAddr(config.TCP.PreferIPV4, config.LocalHost, config.LocalPort)
	if err != nil {
		return nil, common.NewError("invalid local address").Base(err)
	}
	config.LocalAddr = localAddr
	config.LocalIP = localAddr.IP

	remoteAddr, err := convertToAddr(config.TCP.PreferIPV4, config.RemoteHost, config.RemotePort)
	if err != nil {
		return nil, common.NewError("invalid remote address").Base(err)
	}
	config.RemoteAddr = remoteAddr
	config.RemoteIP = remoteAddr.IP

	if config.TLS.FallbackPort != 0 {
		fallbackAddr, err := convertToAddr(config.TCP.PreferIPV4, config.RemoteHost, config.TLS.FallbackPort)
		if err != nil {
			return nil, common.NewError("invalid tls fallback address").Base(err)
		}
		config.TLS.FallbackAddr = fallbackAddr
	}

	if config.TLS.Cipher != "" || config.TLS.CipherTLS13 != "" {
		specifiedSuites := strings.Split(config.TLS.Cipher+":"+config.TLS.CipherTLS13, ":")
		supportedSuites := tls.CipherSuites()
		invalid := false
		for _, specified := range specifiedSuites {
			found := false
			if specified == "" {
				continue
			}
			for _, supported := range supportedSuites {
				if supported.Name == specified {
					config.TLS.CipherSuites = append(config.TLS.CipherSuites, supported.ID)
					found = true
					break
				}
			}
			if !found {
				invalid = true
				log.DefaultLogger.Warn("found invalid cipher name", specified)
				break
			}
		}
		if invalid && len(supportedSuites) >= 1 {
			log.DefaultLogger.Warn("cipher list contains invalid cipher name, ignored")
			log.DefaultLogger.Warn("here's a list of supported ciphers:")
			list := ""
			for _, c := range supportedSuites {
				list += c.Name + ":"
			}
			log.DefaultLogger.Warn(list[0 : len(list)-1])
			config.TLS.CipherSuites = nil
		}
	}

	if config.TLS.HTTPFile != "" {
		payload, err := ioutil.ReadFile(config.TLS.HTTPFile)
		if err != nil {
			log.DefaultLogger.Warn("failed to load http response file", err)
		}
		config.TLS.HTTPResponse = payload
	}

	config.Router.BlockList = []byte{}
	config.Router.ProxyList = []byte{}
	config.Router.BypassList = []byte{}
	for _, path := range config.Router.BlockFiles {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}
		config.Router.BlockList = append(config.Router.BlockList, data...)
		config.Router.BlockList = append(config.Router.BlockList, byte('\n'))
	}

	for _, path := range config.Router.ProxyFiles {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}
		config.Router.ProxyList = append(config.Router.ProxyList, data...)
		config.Router.ProxyList = append(config.Router.ProxyList, byte('\n'))
	}

	for _, path := range config.Router.BypassFiles {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}
		config.Router.BypassList = append(config.Router.BypassList, data...)
		config.Router.BypassList = append(config.Router.BypassList, byte('\n'))
	}

	return &config, nil
}
