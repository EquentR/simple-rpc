// Package tlsutil 提供TLS相关的工具函数
// 包括自签名证书生成等功能
package tlsutil

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"github.com/EquentR/simple-rpc/logger"
	"math/big"
	"net"
	"time"
)

// SelfSigned 生成自签名TLS证书和配置
// hosts: 证书适用的主机名或IP地址列表
// 返回: 证书对象、TLS配置、错误信息
func SelfSigned(hosts []string) (tls.Certificate, *tls.Config, error) {
	logger.Info("Starting to generate self-signed certificate, host list: %v", hosts)

	// 生成RSA私钥
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		logger.Error("Failed to generate RSA private key: %v", err)
		return tls.Certificate{}, nil, err
	}
	logger.Debug("RSA private key generated successfully, key length: 2048 bits")

	// 生成序列号
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		logger.Error("Failed to generate serial number: %v", err)
		return tls.Certificate{}, nil, err
	}
	logger.Debug("Certificate serial number generated successfully: %s", serial.String())

	// Build certificate template
	tmpl := &x509.Certificate{
		SerialNumber:          serial,                                                       // 证书序列号
		Subject:               pkix.Name{CommonName: hosts[0]},                              // 主题信息
		NotBefore:             time.Now().Add(-time.Hour),                                   // 生效时间（1小时前）
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),                         // 过期时间（1年后）
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature, // 密钥用途
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},               // 扩展密钥用途（服务器认证）
		BasicConstraintsValid: true,                                                         // 基本约束有效
	}

	// 添加主机信息到证书
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			// IP地址
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
			logger.Debug("Adding IP address to certificate: %s", h)
		} else {
			// 域名
			tmpl.DNSNames = append(tmpl.DNSNames, h)
			logger.Debug("Adding domain name to certificate: %s", h)
		}
	}

	// 创建证书
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &k.PublicKey, k)
	if err != nil {
		logger.Error("Failed to create certificate: %v", err)
		return tls.Certificate{}, nil, err
	}
	logger.Info("Certificate created successfully")

	// 构建TLS证书对象
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: k}

	// 构建TLS配置
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}}

	logger.Info("Self-signed certificate generation completed")
	return cert, cfg, nil
}
