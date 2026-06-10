package api

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"

	"github.com/nikiv/ansible-ui/internal/config"
	"github.com/nikiv/ansible-ui/internal/model"
)

type samlSP struct {
	sp           *saml.ServiceProvider
	usernameAttr string
	defaultRole  string
}

// newSAML builds a SAML 2.0 Service Provider from the IdP metadata + an SP
// keypair (auto-generated if not supplied).
func newSAML(ctx context.Context, cfg config.SAMLConfig) (*samlSP, error) {
	if cfg.IDPMetadataURL == "" || cfg.RootURL == "" {
		return nil, errors.New("saml needs SAML_IDP_METADATA_URL + SAML_ROOT_URL")
	}
	key, cert, err := samlKeyPair(cfg)
	if err != nil {
		return nil, err
	}
	root, err := url.Parse(strings.TrimRight(cfg.RootURL, "/"))
	if err != nil {
		return nil, err
	}
	idpURL, err := url.Parse(cfg.IDPMetadataURL)
	if err != nil {
		return nil, err
	}
	idpMeta, err := samlsp.FetchMetadata(ctx, &http.Client{Timeout: 10 * time.Second}, *idpURL)
	if err != nil {
		return nil, err
	}
	sp := &saml.ServiceProvider{
		EntityID:          cfg.EntityID, // empty → MetadataURL is used as the entity id
		Key:               key,
		Certificate:       cert,
		MetadataURL:       *root.ResolveReference(&url.URL{Path: "/api/auth/saml/metadata"}),
		AcsURL:            *root.ResolveReference(&url.URL{Path: "/api/auth/saml/acs"}),
		IDPMetadata:       idpMeta,
		AllowIDPInitiated: true,
	}
	role := cfg.DefaultRole
	if role == "" {
		role = model.RoleUser
	}
	return &samlSP{sp: sp, usernameAttr: cfg.UsernameAttr, defaultRole: role}, nil
}

// samlKeyPair parses the configured PEM keypair, or generates an ephemeral
// self-signed one (fine for unsigned AuthnRequests; a stable cert can be
// supplied so the IdP trusts signed requests).
func samlKeyPair(cfg config.SAMLConfig) (*rsa.PrivateKey, *x509.Certificate, error) {
	if cfg.SPCertPEM != "" && cfg.SPKeyPEM != "" {
		pair, err := tls.X509KeyPair([]byte(cfg.SPCertPEM), []byte(cfg.SPKeyPEM))
		if err != nil {
			return nil, nil, err
		}
		key, ok := pair.PrivateKey.(*rsa.PrivateKey)
		if !ok {
			return nil, nil, errors.New("saml SP key must be RSA")
		}
		leaf, err := x509.ParseCertificate(pair.Certificate[0])
		if err != nil {
			return nil, nil, err
		}
		return key, leaf, nil
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "ansible-ui-saml-sp"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}
	_ = pem.Block{} // (PEM only needed if we ever export the keypair)
	return key, cert, nil
}

func (s *Server) handleSAMLMetadata(w http.ResponseWriter, r *http.Request) {
	if s.saml == nil {
		writeErr(w, http.StatusNotFound, "saml not enabled")
		return
	}
	buf, err := xml.MarshalIndent(s.saml.sp.Metadata(), "", "  ")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/samlmetadata+xml")
	_, _ = w.Write(buf)
}

func (s *Server) handleSAMLLogin(w http.ResponseWriter, r *http.Request) {
	if s.saml == nil {
		writeErr(w, http.StatusNotFound, "saml not enabled")
		return
	}
	u, err := s.saml.sp.MakeRedirectAuthenticationRequest("")
	if err != nil {
		writeErr(w, http.StatusBadGateway, "saml authn request: "+err.Error())
		return
	}
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (s *Server) handleSAMLACS(w http.ResponseWriter, r *http.Request) {
	if s.saml == nil {
		writeErr(w, http.StatusNotFound, "saml not enabled")
		return
	}
	assertion, err := s.saml.sp.ParseResponse(r, []string{})
	if err != nil {
		s.log.Warn("saml assertion rejected", "err", err)
		http.Redirect(w, r, "/?sso_error=saml", http.StatusFound)
		return
	}
	attrs := samlAttributes(assertion)
	username := ""
	if s.saml.usernameAttr != "" {
		username = attrs[s.saml.usernameAttr]
	}
	nameID := ""
	if assertion.Subject != nil && assertion.Subject.NameID != nil {
		nameID = assertion.Subject.NameID.Value
	}
	if username == "" {
		username = nameID
	}
	email := firstNonEmpty(attrs["email"], attrs["mail"],
		attrs["urn:oid:0.9.2342.19200300.100.1.3"], attrs["http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"])
	if username == "" {
		username = email
	}
	if username == "" {
		http.Redirect(w, r, "/?sso_error=saml_no_user", http.StatusFound)
		return
	}
	// Account linking by email (like OIDC), else upsert.
	var u *model.User
	if email != "" {
		if existing, gerr := s.store.GetUserByEmail(r.Context(), email); gerr == nil && existing != nil {
			u = existing
		}
	}
	if u == nil {
		u, err = s.store.UpsertExternalUser(r.Context(), username, email, s.saml.defaultRole)
		if err != nil {
			s.handleStoreErr(w, err)
			return
		}
	}
	// Group → role mapping from SAML group attributes (groups / Role / memberOf).
	groups := append(append(samlAttrAll(assertion, "groups"), samlAttrAll(assertion, "Role")...), samlAttrAll(assertion, "memberOf")...)
	s.applyGroupRole(r.Context(), u, groups, s.saml.defaultRole)
	s.issueSession(w, r, u)
	s.recordActivity(r.Context(), u.Username, "auth.login", u.Username, "saml")
	http.Redirect(w, r, "/", http.StatusFound)
}

// samlAttributes flattens the assertion's attribute statements to name→first-value
// (keyed by both Name and FriendlyName).
func samlAttributes(a *saml.Assertion) map[string]string {
	out := map[string]string{}
	if a == nil {
		return out
	}
	for _, st := range a.AttributeStatements {
		for _, at := range st.Attributes {
			if len(at.Values) == 0 {
				continue
			}
			if at.Name != "" {
				out[at.Name] = at.Values[0].Value
			}
			if at.FriendlyName != "" {
				out[at.FriendlyName] = at.Values[0].Value
			}
		}
	}
	return out
}

// samlAttrAll returns every value of an attribute (matched by Name or FriendlyName).
func samlAttrAll(a *saml.Assertion, name string) []string {
	var out []string
	if a == nil {
		return out
	}
	for _, st := range a.AttributeStatements {
		for _, at := range st.Attributes {
			if at.Name == name || at.FriendlyName == name {
				for _, v := range at.Values {
					out = append(out, v.Value)
				}
			}
		}
	}
	return out
}

func (s *Server) samlEnabled() bool { return s.saml != nil }

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
