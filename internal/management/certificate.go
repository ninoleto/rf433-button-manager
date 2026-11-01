package management

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

type certificateFiles struct {
	CA    []byte `json:"ca"`
	CAKey []byte `json:"ca_key"`
	Cert  []byte `json:"certificate"`
	Key   []byte `json:"key"`
}

func serial() *big.Int {
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		panic(err)
	}
	return n
}
func pemCert(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
func pemKey(key *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

func loadCertificate(dir string, hosts []string, renew bool) (certificateFiles, tls.Certificate, error) {
	var files certificateFiles
	var pair tls.Certificate
	path := filepath.Join(dir, "tls.json")
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return files, pair, errors.New("TLS storage must be a regular private file")
	}
	data, err := os.ReadFile(path)
	if err == nil {
		if err = json.Unmarshal(data, &files); err != nil {
			return files, pair, errors.New("invalid TLS storage; preserve it for recovery")
		}
		if err = os.Chmod(path, 0600); err != nil {
			return files, pair, err
		}
	} else if !os.IsNotExist(err) {
		return files, pair, err
	}
	now := time.Now()
	if len(files.CA) == 0 {
		key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			return files, pair, e
		}
		ca := &x509.Certificate{SerialNumber: serial(), Subject: pkix.Name{CommonName: "RF433 Button Manager local CA"}, NotBefore: now.Add(-5 * time.Minute), NotAfter: now.AddDate(10, 0, 0), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign, MaxPathLenZero: true}
		der, e := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
		if e != nil {
			return files, pair, e
		}
		files.CA = pemCert(der)
		files.CAKey, e = pemKey(key)
		if e != nil {
			return files, pair, e
		}
	}
	caBlock, _ := pem.Decode(files.CA)
	keyBlock, _ := pem.Decode(files.CAKey)
	if caBlock == nil || keyBlock == nil {
		return files, pair, errors.New("invalid local certificate authority")
	}
	ca, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return files, pair, err
	}
	caKey, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return files, pair, err
	}
	if !ca.IsCA || now.After(ca.NotAfter.Add(-24*time.Hour)) {
		return files, pair, errors.New("local certificate authority expired; restore or replace through documented recovery")
	}
	pair, err = tls.X509KeyPair(files.Cert, files.Key)
	if err == nil && !renew {
		cert, e := x509.ParseCertificate(pair.Certificate[0])
		if e == nil && cert.NotAfter.After(now.Add(30*24*time.Hour)) && cert.CheckSignatureFrom(ca) == nil {
			valid := true
			for _, host := range hosts {
				if cert.VerifyHostname(host) != nil {
					valid = false
				}
			}
			if valid {
				return files, pair, nil
			}
		}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return files, pair, err
	}
	leaf := &x509.Certificate{SerialNumber: serial(), Subject: pkix.Name{CommonName: "RF433 Button Manager"}, NotBefore: now.Add(-5 * time.Minute), NotAfter: now.AddDate(1, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	for _, host := range hosts {
		if host == "localhost" {
			leaf.DNSNames = append(leaf.DNSNames, host)
			continue
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return files, pair, errors.New("certificate requires an explicit IP address")
		}
		leaf.IPAddresses = append(leaf.IPAddresses, ip)
	}
	der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, caKey)
	if err != nil {
		return files, pair, err
	}
	files.Cert = pemCert(der)
	files.Key, err = pemKey(key)
	if err != nil {
		return files, pair, err
	}
	pair, err = tls.X509KeyPair(files.Cert, files.Key)
	if err != nil {
		return files, pair, err
	}
	data, err = json.Marshal(files)
	if err != nil {
		return files, pair, err
	}
	if err = AtomicPrivate(path, data); err != nil {
		return files, pair, err
	}
	return files, pair, nil
}
