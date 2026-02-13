package mongocontacts

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Meta struct {
	TenantID     primitive.ObjectID
	ConnectionID primitive.ObjectID
}

type ContactUpsert struct {
	InstanceName  string
	ConnectionID  string
	RemoteJid     string
	RemoteLid     string
	PushName      string
	ProfilePicUrl string
	IsGroup       bool
	Phone         string
	UpdatedAt     time.Time
}

type Store struct {
	dbName string
	client *mongo.Client

	colContacts     *mongo.Collection
	colConnections  *mongo.Collection
	metaCacheByInst *xsync.Map[string, cachedMeta]
}

type cachedMeta struct {
	Meta      Meta
	ExpiresAt time.Time
}

const metaCacheTTL = 60 * time.Second

func New(client *mongo.Client, dbName string) *Store {
	if client == nil {
		return nil
	}
	if dbName == "" {
		dbName = "rocketzap"
	}
	db := client.Database(dbName)
	return &Store{
		dbName:          dbName,
		client:          client,
		colContacts:     db.Collection("contacts"),
		colConnections:  db.Collection("whatsappconnections"),
		metaCacheByInst: xsync.NewMap[string, cachedMeta](),
	}
}

var ErrNoConnectionForInstance = errors.New("no whatsapp connection for instanceName")

func (s *Store) ResolveMeta(ctx context.Context, instanceName string) (*Meta, error) {
	if s == nil {
		return nil, nil
	}
	if instanceName == "" {
		return nil, ErrNoConnectionForInstance
	}
	if v, ok := s.metaCacheByInst.Load(instanceName); ok {
		if v.ExpiresAt.After(time.Now()) {
			m := v.Meta
			return &m, nil
		}
	}

	var doc struct {
		ID       primitive.ObjectID `bson:"_id"`
		TenantID primitive.ObjectID `bson:"tenantId"`
	}
	err := s.colConnections.FindOne(ctx, bson.M{"instanceName": instanceName}, options.FindOne().SetProjection(bson.M{
		"_id":      1,
		"tenantId": 1,
	})).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNoConnectionForInstance
		}
		return nil, err
	}

	meta := Meta{
		TenantID:     doc.TenantID,
		ConnectionID: doc.ID,
	}
	s.metaCacheByInst.Store(instanceName, cachedMeta{
		Meta:      meta,
		ExpiresAt: time.Now().Add(metaCacheTTL),
	})
	return &meta, nil
}

func (s *Store) UpsertContact(ctx context.Context, c ContactUpsert) (*Meta, error) {
	if s == nil {
		return nil, nil
	}
	if c.InstanceName == "" || c.RemoteJid == "" {
		return nil, nil
	}

	meta, err := s.ResolveMeta(ctx, c.InstanceName)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, nil
	}

	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = time.Now()
	}

	filter := bson.M{
		"tenantId":  meta.TenantID,
		"remoteJid": c.RemoteJid,
	}

	set := bson.M{
		"isGroup":      c.IsGroup,
		"updatedAt":    c.UpdatedAt,
		"connectionId": meta.ConnectionID,
		"instanceName": c.InstanceName,
	}
	pnJid, lidJid, identityKey := deriveIdentity(c.RemoteJid, c.RemoteLid)
	if pnJid != "" {
		set["remoteJidPn"] = pnJid
	}
	if lidJid != "" {
		set["remoteJidLid"] = lidJid
	}
	if identityKey != "" {
		set["identityKey"] = identityKey
	}
	// Only overwrite pushName/pic if provided.
	if c.PushName != "" {
		set["pushName"] = c.PushName
	}
	if c.ProfilePicUrl != "" {
		set["profilePicUrl"] = c.ProfilePicUrl
	}
	if c.Phone != "" {
		set["phone"] = c.Phone
	}

	setOnInsert := bson.M{
		"tenantId":  meta.TenantID,
		"remoteJid": c.RemoteJid,
	}

	_, err = s.colContacts.UpdateOne(ctx, filter, bson.M{
		"$setOnInsert": setOnInsert,
		"$set":         set,
	}, options.Update().SetUpsert(true))
	return meta, err
}

func deriveIdentity(remoteJid, remoteLid string) (pnJid, lidJid, identityKey string) {
	remoteJid = strings.TrimSpace(strings.ToLower(remoteJid))
	remoteLid = strings.TrimSpace(strings.ToLower(remoteLid))
	if remoteJid == "" {
		return "", "", ""
	}
	if strings.HasSuffix(remoteJid, "@g.us") || strings.HasSuffix(remoteJid, "@broadcast") || strings.HasSuffix(remoteJid, "@newsletter") {
		return "", "", remoteJid
	}
	if strings.HasSuffix(remoteJid, "@s.whatsapp.net") {
		pnJid = remoteJid
	}
	if strings.HasSuffix(remoteJid, "@lid") {
		lidJid = remoteJid
	}
	if strings.HasSuffix(remoteLid, "@lid") {
		lidJid = remoteLid
	}
	if lidJid != "" {
		identityKey = "lid:" + strings.SplitN(lidJid, "@", 2)[0]
		return
	}
	if pnJid != "" {
		identityKey = "pn:" + strings.SplitN(pnJid, "@", 2)[0]
		return
	}
	identityKey = remoteJid
	return
}
