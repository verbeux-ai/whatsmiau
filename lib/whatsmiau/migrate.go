package whatsmiau

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waAdv"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/util/keys"
	"go.uber.org/zap"

	"github.com/verbeux-ai/whatsmiau/env"
	"github.com/verbeux-ai/whatsmiau/models"
	"github.com/verbeux-ai/whatsmiau/server/dto"
)

type baileysBuffer struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type baileysKeyPair struct {
	Private baileysBuffer `json:"private"`
	Public  baileysBuffer `json:"public"`
}

type baileysSignedPreKey struct {
	KeyPair   baileysKeyPair `json:"keyPair"`
	Signature baileysBuffer  `json:"signature"`
	KeyID     uint32         `json:"keyId"`
}

type baileysAccount struct {
	Details             string `json:"details"`
	AccountSignatureKey string `json:"accountSignatureKey"`
	AccountSignature    string `json:"accountSignature"`
	DeviceSignature     string `json:"deviceSignature"`
}

type baileysCreds struct {
	NoiseKey          baileysKeyPair      `json:"noiseKey"`
	SignedIdentityKey baileysKeyPair      `json:"signedIdentityKey"`
	SignedPreKey      baileysSignedPreKey `json:"signedPreKey"`
	RegistrationID    uint32              `json:"registrationId"`
	AdvSecretKey      string              `json:"advSecretKey"`
	Account           baileysAccount      `json:"account"`
	Me                struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		LID  string `json:"lid"`
	} `json:"me"`
	Platform string `json:"platform"`
}

func decodeBuffer(b baileysBuffer) ([]byte, error) {
	return base64.StdEncoding.DecodeString(b.Data)
}

func to32(b []byte) [32]byte {
	var arr [32]byte
	copy(arr[:], b)
	return arr
}

func to64(b []byte) [64]byte {
	var arr [64]byte
	copy(arr[:], b)
	return arr
}

type MigrateResult struct {
	JID       string
	LID       string
	PreKeys   int
	Connected bool
}

func (s *Whatsmiau) Migrate(ctx context.Context, instanceID string, credsRaw json.RawMessage, preKeys []dto.MigrationPreKey) (*MigrateResult, error) {
	var creds baileysCreds
	if err := json.Unmarshal(credsRaw, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse creds: %w", err)
	}

	// Decode keys
	noisePriv, err := decodeBuffer(creds.NoiseKey.Private)
	if err != nil {
		return nil, fmt.Errorf("failed to decode noise key: %w", err)
	}
	identPriv, err := decodeBuffer(creds.SignedIdentityKey.Private)
	if err != nil {
		return nil, fmt.Errorf("failed to decode identity key: %w", err)
	}
	spkPriv, err := decodeBuffer(creds.SignedPreKey.KeyPair.Private)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signed pre-key: %w", err)
	}
	spkSig, err := decodeBuffer(creds.SignedPreKey.Signature)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signed pre-key signature: %w", err)
	}
	advSecret, err := base64.StdEncoding.DecodeString(creds.AdvSecretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode adv secret: %w", err)
	}

	// Decode ADV account
	advDetails, err := base64.StdEncoding.DecodeString(creds.Account.Details)
	if err != nil {
		return nil, fmt.Errorf("failed to decode adv details: %w", err)
	}
	advAccSigKey, err := base64.StdEncoding.DecodeString(creds.Account.AccountSignatureKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode adv account sig key: %w", err)
	}
	advAccSig, err := base64.StdEncoding.DecodeString(creds.Account.AccountSignature)
	if err != nil {
		return nil, fmt.Errorf("failed to decode adv account sig: %w", err)
	}
	advDevSig, err := base64.StdEncoding.DecodeString(creds.Account.DeviceSignature)
	if err != nil {
		return nil, fmt.Errorf("failed to decode adv device sig: %w", err)
	}

	// Parse JIDs
	jid, err := types.ParseJID(creds.Me.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JID %q: %w", creds.Me.ID, err)
	}
	lid, err := types.ParseJID(creds.Me.LID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LID %q: %w", creds.Me.LID, err)
	}

	// Build key pairs from private keys
	noiseKP := keys.NewKeyPairFromPrivateKey(to32(noisePriv))
	identKP := keys.NewKeyPairFromPrivateKey(to32(identPriv))
	spkKP := keys.NewKeyPairFromPrivateKey(to32(spkPriv))

	// Verify public keys match
	noisePub, _ := decodeBuffer(creds.NoiseKey.Public)
	identPub, _ := decodeBuffer(creds.SignedIdentityKey.Public)
	spkPub, _ := decodeBuffer(creds.SignedPreKey.KeyPair.Public)

	if *noiseKP.Pub != to32(noisePub) || *identKP.Pub != to32(identPub) || *spkKP.Pub != to32(spkPub) {
		return nil, fmt.Errorf("public key derivation mismatch: keys are incompatible")
	}

	// Build ADV protobuf
	advAccount := &waAdv.ADVSignedDeviceIdentity{
		Details:             advDetails,
		AccountSignatureKey: advAccSigKey,
		AccountSignature:    advAccSig,
		DeviceSignature:     advDevSig,
	}

	// Build device
	sig64 := to64(spkSig)
	device := &store.Device{
		NoiseKey:    noiseKP,
		IdentityKey: identKP,
		SignedPreKey: &keys.PreKey{
			KeyPair:   *spkKP,
			KeyID:     creds.SignedPreKey.KeyID,
			Signature: &sig64,
		},
		RegistrationID: creds.RegistrationID,
		AdvSecretKey:   advSecret,
		ID:             &jid,
		LID:            lid,
		Account:        advAccount,
		Platform:       creds.Platform,
		PushName:       creds.Me.Name,
	}

	// Save device
	if err := s.container.PutDevice(ctx, device); err != nil {
		return nil, fmt.Errorf("failed to save device: %w", err)
	}
	zap.L().Info("migration: device saved", zap.String("jid", jid.String()))

	// Import pre-keys via raw SQL
	preKeysImported := 0
	if len(preKeys) > 0 {
		preKeysImported, err = s.importPreKeys(ctx, jid.String(), preKeys)
		if err != nil {
			zap.L().Error("migration: failed to import some pre-keys", zap.Error(err))
		}
	}

	// Update instance remoteJID
	if _, err := s.repo.Update(ctx, instanceID, &models.Instance{
		RemoteJID: jid.String(),
	}); err != nil {
		zap.L().Error("migration: failed to update instance remoteJID", zap.Error(err))
	}

	// Create client and connect
	client := whatsmeow.NewClient(device, s.logger)
	client.AddEventHandler(s.Handle(instanceID))
	s.storeClient(instanceID, client)

	connected := false
	if err := client.Connect(); err != nil {
		zap.L().Error("migration: failed to connect", zap.Error(err))
	} else {
		connected = client.IsConnected()
	}

	return &MigrateResult{
		JID:       jid.String(),
		LID:       lid.String(),
		PreKeys:   preKeysImported,
		Connected: connected,
	}, nil
}

func (s *Whatsmiau) importPreKeys(ctx context.Context, jidStr string, preKeys []dto.MigrationPreKey) (int, error) {
	dbURI := env.Env.DBURL
	dialect := env.Env.DBDialect

	db, err := sql.Open(dialect, dbURI)
	if err != nil {
		return 0, fmt.Errorf("failed to open db for pre-keys: %w", err)
	}
	defer db.Close()

	query := "INSERT OR IGNORE INTO whatsmeow_pre_keys (jid, key_id, key, uploaded) VALUES ($1, $2, $3, $4)"
	if dialect == "postgres" {
		query = "INSERT INTO whatsmeow_pre_keys (jid, key_id, key, uploaded) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING"
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	imported := 0
	for _, pk := range preKeys {
		privBytes, err := base64.StdEncoding.DecodeString(pk.Private.Data)
		if err != nil {
			continue
		}
		if _, err := stmt.ExecContext(ctx, jidStr, pk.KeyID, privBytes, true); err != nil {
			continue
		}
		imported++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit: %w", err)
	}
	return imported, nil
}
