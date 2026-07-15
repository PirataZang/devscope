package collectors

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devscope/devscope/internal/core"
)

// CollectSSLCerts reads Let's Encrypt certificates from /etc/letsencrypt/live.
func CollectSSLCerts() []core.SSLCert {
	root := "/etc/letsencrypt/live"
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	var certs []core.SSLCert
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "README" {
			continue
		}
		certPath := filepath.Join(root, e.Name(), "fullchain.pem")
		data, err := os.ReadFile(certPath)
		if err != nil {
			continue
		}
		block, _ := pem.Decode(data)
		if block == nil {
			continue
		}
		x509Cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		daysLeft := int(time.Until(x509Cert.NotAfter).Hours() / 24)
		certs = append(certs, core.SSLCert{
			Domain:    e.Name(),
			Issuer:    x509Cert.Issuer.CommonName,
			ExpiresAt: x509Cert.NotAfter,
			DaysLeft:  daysLeft,
			AutoRenew: true,
		})
	}
	return certs
}

// AssignSSLToProjects attaches SSL info to projects via domain host match.
func AssignSSLToProjects(projects []core.Project, sslCerts []core.SSLCert) {
	for i := range projects {
		projects[i].SSL = nil
	}
	for _, cert := range sslCerts {
		for i, p := range projects {
			for _, d := range p.Domains {
				if strings.EqualFold(d.Host, cert.Domain) || strings.HasSuffix(d.Host, "."+cert.Domain) {
					projects[i].SSL = append(projects[i].SSL, cert)
				}
			}
		}
	}
}
