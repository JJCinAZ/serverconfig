package serverconfig

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// HTTPConfig are values necessary to start up an HTTP/HTTPS server for an application.
// ExternalHostName should be a slice of strings indicating the hostnames of the server, e.g.
//
//	externalhostname:
//	  - www.acme.com
//
// If SkipHostNameTest is not true, then a DNS test for
type HTTPConfig struct {
	SSLBindAddr      string                  `yaml:"sslbindaddr"`
	BindAddr         string                  `yaml:"bindaddr"`
	TemplatePath     string                  `yaml:"templatepath"`
	ExternalHostName []string                `yaml:"externalhostname"`
	SkipHostNameTest bool                    `yaml:"skiphostnametest"`
	Session          HTTPSessionCookieConfig `yaml:"sessioncookie"`
	StaticCert       HTTPStaticCertConfig    `yaml:"static_cert"`
	ACME             HTTPACMEConfig          `yaml:"acme"`
}

type HTTPSessionCookieConfig struct {
	HashKey       string        `yaml:"hashkey" env:"SESSIONHASHKEY"`
	EncryptKey    string        `yaml:"encryptkey" env:"SESSIONENCRYPTKEY"`
	Domain        string        `yaml:"domain"`
	MaxAgeSeconds int           `yaml:"maxageseconds"`
	SameSite      http.SameSite `yaml:"samesite"` // 1 = default, 2 = lax, 3 = strict, 4 = none or "default", "lax", "strict", "none"
	Secure        bool          `yaml:"secure"`   // true if cookie should only be sent over HTTPS
	HttpOnly      bool          `yaml:"httponly"` // true if cookie should not be accessible via JavaScript
}

func (cfg *HTTPSessionCookieConfig) UnmarshalYAML(value *yaml.Node) error {
	type httpSessionCookieConfigYAML struct {
		HashKey       string `yaml:"hashkey"`
		EncryptKey    string `yaml:"encryptkey"`
		Domain        string `yaml:"domain"`
		MaxAgeSeconds int    `yaml:"maxageseconds"`
		SameSite      any    `yaml:"samesite"`
		Secure        bool   `yaml:"secure"`
		HttpOnly      bool   `yaml:"httponly"`
	}
	var (
		raw      httpSessionCookieConfigYAML
		err      error
		sameSite http.SameSite
	)

	err = value.Decode(&raw)
	if err != nil {
		return err
	}

	cfg.HashKey = raw.HashKey
	cfg.EncryptKey = raw.EncryptKey
	cfg.Domain = raw.Domain
	cfg.MaxAgeSeconds = raw.MaxAgeSeconds
	cfg.Secure = raw.Secure
	cfg.HttpOnly = raw.HttpOnly

	if raw.SameSite == nil {
		return nil
	}

	sameSite, err = parseHTTPSameSite(raw.SameSite)
	if err != nil {
		return err
	}
	cfg.SameSite = sameSite

	return nil
}

func parseHTTPSameSite(raw any) (http.SameSite, error) {
	var (
		s         string
		parsedInt int
		err       error
	)

	switch value := raw.(type) {
	case int:
		return httpSameSiteFromInt(value)
	case int64:
		return httpSameSiteFromInt(int(value))
	case uint64:
		return httpSameSiteFromInt(int(value))
	case string:
		s = strings.TrimSpace(value)
		parsedInt, err = strconv.Atoi(s)
		if err == nil {
			return httpSameSiteFromInt(parsedInt)
		}
		return httpSameSiteFromString(s)
	default:
		return 0, fmt.Errorf("invalid samesite type %T (expected int or string)", raw)
	}
}

func httpSameSiteFromInt(value int) (http.SameSite, error) {
	switch value {
	case int(http.SameSiteDefaultMode):
		return http.SameSiteDefaultMode, nil
	case int(http.SameSiteLaxMode):
		return http.SameSiteLaxMode, nil
	case int(http.SameSiteStrictMode):
		return http.SameSiteStrictMode, nil
	case int(http.SameSiteNoneMode):
		return http.SameSiteNoneMode, nil
	default:
		return 0, fmt.Errorf("invalid samesite integer %d (expected 1, 2, 3, or 4)", value)
	}
}

func httpSameSiteFromString(value string) (http.SameSite, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "default", "defaultmode", "samesitedefaultmode":
		return http.SameSiteDefaultMode, nil
	case "lax", "laxmode", "samesitelaxmode":
		return http.SameSiteLaxMode, nil
	case "strict", "strictmode", "samesitestrictmode":
		return http.SameSiteStrictMode, nil
	case "none", "nonemode", "samesitenonemode":
		return http.SameSiteNoneMode, nil
	default:
		return 0, fmt.Errorf("invalid samesite string %q (expected Default, Lax, Strict, or None)", value)
	}
}

type HTTPStaticCertConfig struct {
	SSLCertFile       string `yaml:"certfile"`
	SSLPrivateKeyFile string `yaml:"privatekeyfile"`
}

type HTTPACMEConfig struct {
	Email     string `yaml:"email"`
	CADirURL  string `yaml:"cadirurl"`
	DiskCache string `yaml:"diskcache"`
}

func (cfg *HTTPConfig) Verify() error {
	var err error

	if len(cfg.ExternalHostName) == 0 || len(cfg.ExternalHostName[0]) == 0 {
		return fmt.Errorf("missing at least one externalhostname in configuration")
	}

	if !cfg.SkipHostNameTest {
		err = TestExternalHostName(cfg.ExternalHostName[0])
		if err != nil {
			return fmt.Errorf("[TestExternalHostName] failed: %w", err)
		}
	}

	if len(cfg.StaticCert.SSLCertFile) == 0 || len(cfg.StaticCert.SSLPrivateKeyFile) == 0 {
		if len(cfg.ACME.Email) == 0 {
			return fmt.Errorf("ACME certificates are enabled, but the config is missing http.acme.email value for email address for registration")
		}
		if len(cfg.ACME.DiskCache) == 0 {
			return fmt.Errorf("ACME certificates are enabled, but the config is missing http.acme.diskcache value caching certificates")
		}
	}

	return nil
}

// TestExternalHostName will check to see if the IPv4 address to which hostname resolves is, in fact, the IP address
// to which this host is mapped, or is using.  The IP address is NOT that assigned to an interface on the host,
// rather, a test probe with an outside server is conducted to see to which IP address the host might be NAT'ed.
// This isn't a guarantee that the host might be NAT'ed on inbound traffic to more than one IP or that the outbound
// IP isn't the same as the inbound IP.  The host can be dual-homed (IPv4 and IPv6) but no tests are conducted on the
// IPv6 address(es) or AAAA DNS names.
func TestExternalHostName(hostname string) error {
	var (
		x          bytes.Buffer
		externalIP string
		providers  = [4]string{
			"https://ipv4.whatismyip.akamai.com",
			"https://ipv4.myexternalip.com/raw",
			"https://ipecho.net/plain",
			"https://eth0.me",
		}
		ipAddrs []string
		client  *http.Client
		resp    *http.Response
		err     error
		i       int
	)

	ipAddrs, err = net.LookupHost(hostname)
	if err != nil {
		return err
	}

	for i = 0; i < len(providers); i++ {
		client = &http.Client{Timeout: 4 * time.Second}
		resp, err = client.Get(providers[i])
		if err != nil {
			continue
		}

		_, _ = io.Copy(&x, resp.Body)
		_ = resp.Body.Close()
		externalIP = strings.TrimSpace(x.String())
		x.Reset()
		if net.ParseIP(externalIP) != nil {
			break
		}
	}

	if len(externalIP) == 0 {
		return fmt.Errorf("unable to get external IP from any provider")
	}

	for i = 0; i < len(ipAddrs); i++ {
		if ipAddrs[i] == externalIP {
			return nil
		}
	}

	return fmt.Errorf("external IP %s doesn't match any value in DNS (%s) for host name %s",
		externalIP, strings.Join(ipAddrs, ", "), hostname)
}
