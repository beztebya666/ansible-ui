package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/nikiv/ansible-ui/internal/model"
)

type credentialInput struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Login          string `json:"login"`
	SSHPrivateKey  string `json:"sshPrivateKey"`
	SSHCertificate string `json:"sshCertificate"` // OpenSSH user cert (public); stored as metadata, not secret
	Passphrase     string `json:"passphrase"`
	Password       string `json:"password"`
	VaultPassword  string `json:"vaultPassword"`
	ServiceAccount string `json:"serviceAccount"`
	Personal       bool   `json:"personal"` // owned by the creating user (visible only to them)
}

func (in credentialInput) hasSecret() bool {
	return in.SSHPrivateKey != "" || in.Password != "" || in.VaultPassword != "" || in.Passphrase != "" || in.ServiceAccount != ""
}

func (in credentialInput) secret() model.CredentialSecret {
	return model.CredentialSecret{
		SSHPrivateKey:  in.SSHPrivateKey,
		Passphrase:     in.Passphrase,
		Password:       in.Password,
		VaultPassword:  in.VaultPassword,
		ServiceAccount: in.ServiceAccount,
	}
}

func (s *Server) handleListCredentials(w http.ResponseWriter, r *http.Request) {
	uid := ""
	if u := currentUser(r); u != nil {
		uid = u.ID
	}
	creds, err := s.store.ListCredentials(r.Context(), projectScope(r), uid)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, creds)
}

func (s *Server) handleCreateCredential(w http.ResponseWriter, r *http.Request) {
	var in credentialInput
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if in.Name == "" || in.Type == "" {
		writeErr(w, http.StatusBadRequest, "name and type are required")
		return
	}
	blob, err := s.encryptSecret(in.secret())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "encrypt: "+err.Error())
		return
	}
	c := &model.Credential{Name: in.Name, Type: in.Type, Login: in.Login, SSHCertificate: strings.TrimSpace(in.SSHCertificate), ProjectID: scopePtr(projectScope(r))}
	if in.Personal { // personal: owned by the creating user, never project-scoped
		if u := currentUser(r); u != nil {
			uid := u.ID
			c.OwnerUserID = &uid
			c.ProjectID = nil
		}
	}
	if err := s.store.CreateCredential(r.Context(), c, blob); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "credential.created", c.Name, c.Type)
	c.HasSecret = in.hasSecret()
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handleUpdateCredential(w http.ResponseWriter, r *http.Request) {
	c, err := s.store.GetCredential(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	var in credentialInput
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if in.Name != "" {
		c.Name = in.Name
	}
	if in.Type != "" {
		c.Type = in.Type
	}
	c.Login = in.Login
	c.SSHCertificate = strings.TrimSpace(in.SSHCertificate)
	updateSecret := in.hasSecret()
	var blob []byte
	if updateSecret {
		if blob, err = s.encryptSecret(in.secret()); err != nil {
			writeErr(w, http.StatusInternalServerError, "encrypt: "+err.Error())
			return
		}
	}
	if err := s.store.UpdateCredential(r.Context(), c, blob, updateSecret); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "credential.updated", c.Name, c.Type)
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleDeleteCredential(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := id
	if c, err := s.store.GetCredential(r.Context(), id); err == nil {
		name = c.Name
	}
	if err := s.store.DeleteCredential(r.Context(), id); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "credential.deleted", name, "")
	w.WriteHeader(http.StatusNoContent)
}

// handleCredentialPubKey derives + returns the SSH public key (authorized_keys
// format) for an SSH credential, so an operator can install it on target hosts /
// register it as a git deploy key. The private key never leaves the server.
func (s *Server) handleCredentialPubKey(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id := r.PathValue("id")
	cred, err := s.store.GetCredential(r.Context(), id)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	sec, err := s.credentialSecret(r.Context(), id)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if strings.TrimSpace(sec.SSHPrivateKey) == "" {
		writeErr(w, http.StatusBadRequest, "credential has no SSH private key")
		return
	}
	var signer ssh.Signer
	if sec.Passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(sec.SSHPrivateKey), []byte(sec.Passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey([]byte(sec.SSHPrivateKey))
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, "parse private key: "+err.Error())
		return
	}
	pub := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	s.recordSecretAccess(r.Context(), "", "pubkey", id, cred.Name, "derived public key ("+signer.PublicKey().Type()+")")
	writeJSON(w, http.StatusOK, map[string]string{
		"publicKey": pub + " ansible-ui-" + cred.Name,
		"type":      signer.PublicKey().Type(),
	})
}

// ---- secret crypto -----------------------------------------------------

func (s *Server) encryptSecret(secret model.CredentialSecret) ([]byte, error) {
	plain, err := json.Marshal(secret)
	if err != nil {
		return nil, err
	}
	return s.cipher.Encrypt(plain)
}

// credentialSecret decrypts a credential's secret material (server-side use).
func (s *Server) credentialSecret(ctx context.Context, id string) (*model.CredentialSecret, error) {
	blob, err := s.store.GetCredentialSecret(ctx, id)
	if err != nil {
		return nil, err
	}
	var sec model.CredentialSecret
	if len(blob) == 0 {
		return &sec, nil
	}
	plain, err := s.cipher.Decrypt(blob)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(plain, &sec); err != nil {
		return nil, err
	}
	return &sec, nil
}
