package whatsmiau

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/verbeux-ai/whatsmiau/env"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/lib/storage/gcs"
	"github.com/verbeux-ai/whatsmiau/lib/storage/s3"
	"github.com/verbeux-ai/whatsmiau/models"
	"github.com/verbeux-ai/whatsmiau/repositories/instances"
	"github.com/verbeux-ai/whatsmiau/repositories/messages"
	"github.com/verbeux-ai/whatsmiau/repositories/mongocontacts"
	"github.com/verbeux-ai/whatsmiau/repositories/mongomessages"
	"github.com/verbeux-ai/whatsmiau/services"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"google.golang.org/protobuf/proto"
)

type Whatsmiau struct {
	clients          *xsync.Map[string, *whatsmeow.Client]
	container        *sqlstore.Container
	logger           waLog.Logger
	repo             interfaces.InstanceRepository
	qrCache          *xsync.Map[string, string]
	observerRunning  *xsync.Map[string, bool]
	instanceCache    *xsync.Map[string, models.Instance]
	lockConnection   *xsync.Map[string, *sync.Mutex]
	emitter          chan emitter
	httpClient       *http.Client
	fileStorage      interfaces.Storage
	messageStore     *messages.Store
	mongoMessages    *mongomessages.Store
	mongoContacts    *mongocontacts.Store
	selfPushName     *xsync.Map[string, string]
	handlerSemaphore chan struct{}
}

var instance *Whatsmiau
var mu = &sync.Mutex{}
var devicePropsMu sync.Mutex

func Get() *Whatsmiau {
	mu.Lock()
	defer mu.Unlock()
	return instance
}

func LoadMiau(ctx context.Context, container *sqlstore.Container) {
	mu.Lock()
	defer mu.Unlock()
	deviceStore, err := container.GetAllDevices(ctx)
	if err != nil {
		panic(err)
	}

	level := "INFO"
	if env.Env.DebugWhatsmeow {
		level = "DEBUG"
	}

	repo := instances.NewRedis(services.Redis())
	instanceList, err := repo.List(ctx, "")
	if err != nil {
		zap.L().Fatal("failed to list instances", zap.Error(err))
	}

	instanceByRemoteJid := make(map[string]models.Instance)
	for _, inst := range instanceList {
		if len(inst.RemoteJID) <= 0 {
			continue
		}

		instanceByRemoteJid[inst.RemoteJID] = inst
	}

	clients := xsync.NewMap[string, *whatsmeow.Client]()

	clientLog := waLog.Stdout("Client", level, false)
	zap.L().Info("starting auto-reconnect", zap.Int("device_count", len(deviceStore)), zap.Int("instance_count", len(instanceByRemoteJid)))

	for _, device := range deviceStore {
		client := whatsmeow.NewClient(device, clientLog)
		if client.Store.ID == nil {
			zap.L().Error("device without id on db", zap.Any("device", device))
			continue
		}

		jidStr := client.Store.ID.String()
		instanceFound, ok := instanceByRemoteJid[jidStr]
		if ok {
			zap.L().Info("found matching instance for device", zap.String("jid", jidStr), zap.String("instance_id", instanceFound.ID))
			configProxy(client, instanceFound.InstanceProxy)
			clients.Store(instanceFound.ID, client)
			if err := client.Connect(); err != nil {
				zap.L().Error("failed to connect device", zap.Error(err), zap.String("jid", jidStr), zap.String("instance_id", instanceFound.ID))
				// Remove from clients map if connection failed
				clients.Delete(instanceFound.ID)
			} else {
				zap.L().Info("device reconnected successfully", zap.String("jid", jidStr), zap.String("instance_id", instanceFound.ID))
			}
			continue
		}

		zap.L().Warn("no matching instance found for device, logging out", zap.String("jid", jidStr))
		if err := client.Logout(context.TODO()); err != nil {
			zap.L().Error("failed to logout", zap.Error(err), zap.String("jid", jidStr))
		}
		if client.Store != nil && client.Store.ID != nil {
			if err := container.DeleteDevice(context.Background(), client.Store); err != nil {
				zap.L().Error("failed to delete device", zap.Error(err))
			}
		}
	}

	var storage interfaces.Storage
	if env.Env.S3Enabled {
		storage, err = s3.New(context.Background(), s3.Options{
			Endpoint:       env.Env.S3Endpoint,
			Region:         env.Env.S3Region,
			Bucket:         env.Env.S3Bucket,
			AccessKey:      env.Env.S3AccessKey,
			SecretKey:      env.Env.S3SecretKey,
			UseSSL:         env.Env.S3UseSSL,
			ForcePathStyle: env.Env.S3ForcePathStyle,
			PublicURL:      env.Env.S3PublicURL,
		})
		if err != nil {
			zap.L().Panic("failed to create S3 storage", zap.Error(err))
		}
	} else if env.Env.GCSEnabled {
		storage, err = gcs.New(env.Env.GCSBucket)
		if err != nil {
			zap.L().Panic("failed to create GCS storage", zap.Error(err))
		}
	}

	instance = &Whatsmiau{
		clients:         clients,
		container:       container,
		logger:          clientLog,
		repo:            repo,
		qrCache:         xsync.NewMap[string, string](),
		instanceCache:   xsync.NewMap[string, models.Instance](),
		observerRunning: xsync.NewMap[string, bool](),
		lockConnection:  xsync.NewMap[string, *sync.Mutex](),
		emitter:         make(chan emitter, env.Env.EmitterBufferSize),
		httpClient: &http.Client{
			Timeout: time.Second * 30, // TODO: load from env
		},
		fileStorage:      storage,
		messageStore:     services.MessageStore(),
		mongoMessages:    mongomessages.New(services.Mongo(), env.Env.MongoDB),
		mongoContacts:    mongocontacts.New(services.Mongo(), env.Env.MongoDB),
		selfPushName:     xsync.NewMap[string, string](),
		handlerSemaphore: make(chan struct{}, env.Env.HandlerSemaphoreSize),
	}

	go instance.startEmitter()

	// Populate instance cache and add event handlers for reconnected clients
	for jidStr, inst := range instanceByRemoteJid {
		if client, ok := clients.Load(inst.ID); ok {
			instance.instanceCache.Store(inst.ID, inst)
			zap.L().Info("registering event handler", zap.String("jid", jidStr), zap.String("instance_id", inst.ID))
			client.AddEventHandler(instance.Handle(inst.ID))
		}
	}

}

func (s *Whatsmiau) Connect(ctx context.Context, id string) (string, error) {
	client, err := s.generateClient(ctx, id)
	if err != nil {
		return "", err
	}
	if client == nil {
		return "", nil
	}

	if qr, ok := s.qrCache.Load(id); ok {
		return qr, nil
	}

	qrCode, err := s.observeAndQrCode(ctx, id, client)
	if err != nil {
		return "", err
	}

	return qrCode, nil
}

func (s *Whatsmiau) getConnectionLock(id string) *sync.Mutex {
	lock, _ := s.lockConnection.LoadOrCompute(id, func() (*sync.Mutex, bool) {
		return &sync.Mutex{}, false
	})
	return lock
}

func (s *Whatsmiau) generateClient(ctx context.Context, id string) (*whatsmeow.Client, error) {
	lock := s.getConnectionLock(id)
	lock.Lock()
	defer lock.Unlock()

	client, ok := s.clients.Load(id)
	if !ok {
		device := s.container.NewDevice()
		client = whatsmeow.NewClient(device, s.logger)
		s.clients.Store(id, client)
	}

	// trying recover existent connection
	if s.hasSomeDevice(client) {
		if instanceFound := s.getInstanceCached(id); instanceFound != nil {
			configProxy(client, instanceFound.InstanceProxy)
		}

		if client.IsLoggedIn() {
			return nil, nil
		}

		if err := client.Connect(); err == nil {
			if client.IsLoggedIn() {
				return nil, nil
			}
		}

		s.clients.Delete(id)
		if err := s.deleteDeviceIfExists(ctx, client); err != nil {
			zap.L().Error("failed to hard logout", zap.Error(err))
			return nil, err
		}

		device := s.container.NewDevice()
		client = whatsmeow.NewClient(device, s.logger)
		s.clients.Store(id, client) // replaces old client
	}

	return client, nil
}

func (s *Whatsmiau) hasSomeDevice(client *whatsmeow.Client) bool {
	noStore := client.Store == nil
	if noStore {
		return false
	}

	noDevice := client.Store.ID == nil
	if noDevice {
		return false
	}

	return true
}

func (s *Whatsmiau) observeConnection(client *whatsmeow.Client, id string) {
	if _, ok := s.observerRunning.Load(id); ok {
		zap.L().Debug("observer connection already running", zap.String("id", id))
		return
	}

	zap.L().Debug("starting observer connection", zap.String("id", id))
	s.observerRunning.Store(id, true)
	defer func() {
		zap.L().Debug("stopping observer connection", zap.String("id", id))
		s.observerRunning.Delete(id)
		s.qrCache.Delete(id)
		s.lockConnection.Delete(id)
	}()

	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute*2)
	qrChan, err := client.GetQRChannel(ctx)
	if err != nil {
		zap.L().Error("failed to observe QR Code", zap.Error(err))
		s.clients.Delete(id)
		if err := s.deleteDeviceIfExists(context.TODO(), client); err != nil {
			zap.L().Error("failed to cleanup device after GetQRChannel error", zap.String("id", id), zap.Error(err))
		}
		return
	}

	instanceFound := s.getInstance(id)
	if instanceFound != nil {
		configProxy(client, instanceFound.InstanceProxy)
	}

	// Configure history sync based on instance options before connecting.
	// DeviceProps is a package-level variable, so we use a mutex to prevent
	// race conditions when multiple instances connect concurrently.
	devicePropsMu.Lock()
	if instanceFound != nil && instanceFound.SyncFullHistory {
		store.DeviceProps.RequireFullSync = proto.Bool(true)
		store.DeviceProps.HistorySyncConfig = &waCompanionReg.DeviceProps_HistorySyncConfig{
			FullSyncDaysLimit:                        proto.Uint32(3650),
			FullSyncSizeMbLimit:                      proto.Uint32(10240),
			StorageQuotaMb:                           proto.Uint32(10240),
			InlineInitialPayloadInE2EeMsg:            proto.Bool(true),
			SupportCallLogHistory:                    proto.Bool(true),
			SupportBotUserAgentChatHistory:           proto.Bool(true),
			SupportCagReactionsAndPolls:              proto.Bool(true),
			SupportBizHostedMsg:                      proto.Bool(true),
			SupportRecentSyncChunkMessageCountTuning: proto.Bool(true),
			SupportHostedGroupMsg:                    proto.Bool(true),
			SupportFbidBotChatHistory:                proto.Bool(true),
			SupportMessageAssociation:                proto.Bool(true),
			SupportGroupHistory:                      proto.Bool(true),
		}
	} else if instanceFound != nil && instanceFound.SyncRecentHistory {
		store.DeviceProps.RequireFullSync = proto.Bool(false)
		store.DeviceProps.HistorySyncConfig = &waCompanionReg.DeviceProps_HistorySyncConfig{
			RecentSyncDaysLimit:                      proto.Uint32(30),
			StorageQuotaMb:                           proto.Uint32(10240),
			InlineInitialPayloadInE2EeMsg:            proto.Bool(true),
			SupportCallLogHistory:                    proto.Bool(true),
			SupportBotUserAgentChatHistory:           proto.Bool(true),
			SupportCagReactionsAndPolls:              proto.Bool(true),
			SupportBizHostedMsg:                      proto.Bool(true),
			SupportRecentSyncChunkMessageCountTuning: proto.Bool(true),
			SupportHostedGroupMsg:                    proto.Bool(true),
			SupportFbidBotChatHistory:                proto.Bool(true),
			SupportMessageAssociation:                proto.Bool(true),
			SupportGroupHistory:                      proto.Bool(true),
		}
	} else {
		store.DeviceProps.RequireFullSync = proto.Bool(false)
	}

	connectErr := client.Connect()
	devicePropsMu.Unlock()

	if connectErr != nil {
		zap.L().Error("failed to connect connected device", zap.Error(connectErr))
		s.clients.Delete(id)
		if err := s.deleteDeviceIfExists(context.TODO(), client); err != nil {
			zap.L().Error("failed to cleanup device after Connect error", zap.String("id", id), zap.Error(err))
		}
		return
	}

	zap.L().Debug("waiting for QR channel event", zap.String("id", id))
	for {
		select {
		case <-ctx.Done(): // QR code expiration
			zap.L().Debug("context ", zap.String("id", id), zap.Error(ctx.Err()))
			if err := s.deleteDeviceIfExists(context.TODO(), client); err != nil {
				zap.L().Error("failed to hard logout", zap.String("id", id), zap.Error(err))
			}
			s.clients.Delete(id)
			return
		case evt, ok := <-qrChan:
			if !ok || evt.Event == "error" || evt.Event == "timeout" { // closed qr chan
				zap.L().Debug("QR channel closed", zap.String("id", id), zap.Any("evt", evt))
				cancel()
				continue
			}
			zap.L().Debug("received QR channel event", zap.String("id", id), zap.Any("evt", evt))
			if evt.Event == "code" {
				s.qrCache.Store(id, evt.Code)
				continue
			}

			if evt.Event == "success" || evt.Event == "logged_in" {
				if client.Store.ID == nil {
					zap.L().Error("jid is nil after login", zap.String("id", id), zap.Any("evt", evt))
					cancel()
					continue
				}

				zap.L().Info("device connected successfully", zap.String("id", id))
				client.RemoveEventHandlers()
				client.AddEventHandler(s.Handle(id))
				if _, err := s.repo.Update(context.Background(), id, &models.Instance{
					RemoteJID: client.Store.ID.String(),
				}); err != nil {
					zap.L().Error("failed to update instance after login", zap.Error(err))
				}
				s.qrCache.Delete(id)
				return
			}

			zap.L().Error("unknown event", zap.String("id", id), zap.Any("evt", evt))
		}
	}
}

func (s *Whatsmiau) observeAndQrCode(ctx context.Context, id string, client *whatsmeow.Client) (string, error) {
	ctx, c := context.WithTimeout(ctx, 15*time.Second)
	defer c()

	zap.L().Debug("starting observe and qr code", zap.String("id", id))
	go s.observeConnection(client, id)

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			qrCode, ok := s.qrCache.Load(id)
			if ok && len(qrCode) > 0 {
				zap.L().Debug("got qr code from cache", zap.String("id", id))
				return qrCode, nil
			}
		case <-ctx.Done():
			zap.L().Debug("observe and qr code context done", zap.String("id", id), zap.Error(ctx.Err()))
			return "", ctx.Err()
		}
	}
}

func (s *Whatsmiau) deleteDeviceIfExists(ctx context.Context, client *whatsmeow.Client) error {
	if client.IsLoggedIn() {
		if err := client.Logout(ctx); err != nil {
			zap.L().Error("failed to logout", zap.Error(err))
			return err
		}
	}

	if client.Store != nil && client.Store.ID != nil {
		if err := s.container.DeleteDevice(ctx, client.Store); err != nil {
			zap.L().Error("failed to delete device", zap.Error(err))
			return err
		}
	}

	return nil
}

func (s *Whatsmiau) Status(id string) (Status, error) {
	client, ok := s.clients.Load(id)
	if !ok {
		return Closed, nil
	}

	if client.IsConnected() && client.IsLoggedIn() {
		return Connected, nil
	}

	// If not connected, but we have a QR code, the state is QrCode
	if _, ok := s.qrCache.Load(id); ok && client.IsConnected() {
		return QrCode, nil
	}

	if client.IsLoggedIn() {
		return Connecting, nil
	}

	return Closed, nil
}

func (s *Whatsmiau) Logout(ctx context.Context, id string) error {
	lock := s.getConnectionLock(id)
	lock.Lock()
	defer lock.Unlock()

	client, ok := s.clients.Load(id)
	if !ok {
		zap.L().Warn("logout: client does not exist", zap.String("id", id))
		return nil
	}

	s.clients.Delete(id)
	return s.deleteDeviceIfExists(ctx, client)
}

func (s *Whatsmiau) Disconnect(id string) error {
	lock := s.getConnectionLock(id)
	lock.Lock()
	defer lock.Unlock()

	client, ok := s.clients.Load(id)
	if !ok {
		zap.L().Warn("failed to disconnect (device not loaded)", zap.String("id", id))
		return nil
	}

	client.Disconnect()
	s.qrCache.Delete(id)
	return nil
}

func (s *Whatsmiau) GetJidLid(ctx context.Context, id string, jid types.JID) (string, string) {
	newJid, newLid := s.extractJidLid(ctx, id, jid)
	if strings.HasSuffix(newJid, "@lid") {
		newLid = newJid
	}

	return newJid, newLid
}

func (s *Whatsmiau) extractJidLid(ctx context.Context, id string, jid types.JID) (string, string) {
	client, ok := s.clients.Load(id)
	if !ok {
		return jid.ToNonAD().String(), ""
	}

	if jid.Server == types.DefaultUserServer {
		lid, err := client.Store.LIDs.GetLIDForPN(ctx, jid)
		if err != nil {
			zap.L().Warn("failed to get lid from store", zap.String("id", id), zap.Error(err))
		}

		return jid.ToNonAD().String(), lid.ToNonAD().String()
	}

	if jid.Server == types.HiddenUserServer {
		lidString := jid.ToNonAD().String()
		pnJID, err := client.Store.LIDs.GetPNForLID(ctx, jid)
		if err != nil {
			zap.L().Warn("failed to get pn for lid", zap.Stringer("lid", jid), zap.Error(err))
			return jid.ToNonAD().String(), lidString
		}

		if !pnJID.IsEmpty() {
			return pnJID.ToNonAD().String(), lidString
		}

		return lidString, lidString
	}

	return jid.ToNonAD().String(), ""
}
