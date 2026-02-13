package mongomessages

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

type MessageUpsert struct {
	InstanceName    string
	ConnectionID    string
	RemoteJid       string
	RemoteLid       string
	MessageID       string
	FromMe          bool
	PushName        string
	Participant     string
	MessageType     string
	Body            string
	MediaKey        string
	MediaURL        string
	MediaMimetype   string
	MediaFileName   string
	QuotedMessageID string
	Status          string
	Timestamp       time.Time
	Reactions       []any
}

type Store struct {
	dbName string
	client *mongo.Client

	colMessages     *mongo.Collection
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
		colMessages:     db.Collection("messages"),
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

func (s *Store) UpsertMessage(ctx context.Context, msg MessageUpsert) error {
	if s == nil {
		return nil
	}
	if msg.InstanceName == "" || msg.MessageID == "" || msg.RemoteJid == "" {
		return nil
	}

	meta, err := s.ResolveMeta(ctx, msg.InstanceName)
	if err != nil {
		return err
	}
	if meta == nil {
		return nil
	}

	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	if msg.Status == "" {
		if msg.FromMe {
			msg.Status = "sent"
		} else {
			msg.Status = "received"
		}
	}
	if msg.MessageType == "" {
		msg.MessageType = "unknown"
	}

	filter := bson.M{
		"tenantId":     meta.TenantID,
		"connectionId": meta.ConnectionID,
		"messageId":    msg.MessageID,
	}

	setOnInsert := bson.M{
		"tenantId":     meta.TenantID,
		"connectionId": meta.ConnectionID,
		"instanceName": msg.InstanceName,
		"messageId":    msg.MessageID,
	}

	set := bson.M{
		"remoteJid":    msg.RemoteJid,
		"fromMe":       msg.FromMe,
		"pushName":     msg.PushName,
		"participant":  msg.Participant,
		"messageType":  msg.MessageType,
		"body":         msg.Body,
		"status":       msg.Status,
		"timestamp":    msg.Timestamp,
	}
	pnJid, lidJid, identityKey := deriveIdentity(msg.RemoteJid, msg.RemoteLid)
	if pnJid != "" {
		set["remoteJidPn"] = pnJid
	}
	if lidJid != "" {
		set["remoteJidLid"] = lidJid
	}
	if identityKey != "" {
		set["identityKey"] = identityKey
	}

	if msg.MediaURL != "" {
		set["mediaUrl"] = msg.MediaURL
	}
	if msg.MediaKey != "" {
		set["mediaKey"] = msg.MediaKey
	}
	if msg.MediaMimetype != "" {
		set["mediaMimetype"] = msg.MediaMimetype
	}
	if msg.MediaFileName != "" {
		set["mediaFileName"] = msg.MediaFileName
	}
	if msg.QuotedMessageID != "" {
		set["quotedMessageId"] = msg.QuotedMessageID
	}
	if msg.Reactions != nil {
		set["reactions"] = msg.Reactions
	}

	_, err = s.colMessages.UpdateOne(ctx, filter, bson.M{
		"$setOnInsert": setOnInsert,
		"$set":         set,
	}, options.Update().SetUpsert(true))
	return err
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

func (s *Store) UpdateStatus(ctx context.Context, instanceName, messageID, status string) error {
	if s == nil {
		return nil
	}
	if instanceName == "" || messageID == "" || status == "" {
		return nil
	}
	meta, err := s.ResolveMeta(ctx, instanceName)
	if err != nil {
		return err
	}
	if meta == nil {
		return nil
	}

	filter := bson.M{
		"tenantId":     meta.TenantID,
		"connectionId": meta.ConnectionID,
		"messageId":    messageID,
	}
	_, err = s.colMessages.UpdateOne(ctx, filter, bson.M{"$set": bson.M{"status": status}})
	return err
}
