package whatsmiau

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/verbeux-ai/whatsmiau/env"
	"github.com/verbeux-ai/whatsmiau/interfaces"
	"github.com/verbeux-ai/whatsmiau/lib/storage/gcs"
	"github.com/verbeux-ai/whatsmiau/models"
	"github.com/verbeux-ai/whatsmiau/repositories/instances"
	"github.com/verbeux-ai/whatsmiau/services"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type Whatsmiau struct {
	clients            *xsync.Map[string, *whatsmeow.Client]
	container          *sqlstore.Container
	logger             waLog.Logger
	repo               interfaces.InstanceRepository
	qrCache            *xsync.Map[string, string]
	pairingCache       *xsync.Map[string, string]
	observerRunning    *xsync.Map[string, *whatsmeow.Client]
	instanceCache      *xsync.Map[string, models.Instance]
	lockConnection     *xsync.Map[string, *sync.Mutex]
	connectPhoneNumber *xsync.Map[string, string]
	emitter            chan emitter
	httpClient         *http.Client
	fileStorage        interfaces.Storage
	handlerSemaphore   chan struct{}
}

var instance *Whatsmiau
var mu = &sync.Mutex{}

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
	for _, device := range deviceStore {
		client := whatsmeow.NewClient(device, clientLog)
		if client.Store.ID == nil {
			zap.L().Error("device without id on db", zap.Any("device", device))
			continue
		}

		instanceFound, ok := instanceByRemoteJid[client.Store.ID.String()]
		if ok {
			configProxy(client, instanceFound.InstanceProxy)
			clients.Store(instanceFound.ID, client)
			if err := client.Connect(); err != nil {
				jid := ""
				if client.Store != nil && client.Store.ID != nil {
					jid = client.Store.ID.String()
				}
				zap.L().Error("failed to connect connected device", zap.Error(err), zap.String("jid", jid))
			}
			continue
		}

		if err := client.Logout(context.TODO()); err != nil {
			jid := ""
			if client.Store != nil && client.Store.ID != nil {
				jid = client.Store.ID.String()
			}
			zap.L().Error("failed to logout", zap.Error(err), zap.String("jid", jid))
		}
		if client.Store != nil && client.Store.ID != nil {
			if err := container.DeleteDevice(context.Background(), client.Store); err != nil {
				zap.L().Error("failed to delete device", zap.Error(err))
			}
		}
	}

	var storage interfaces.Storage
	if env.Env.GCSEnabled {
		storage, err = gcs.New(env.Env.GCSBucket)
		if err != nil {
			zap.L().Panic("failed to create GCS storage", zap.Error(err))
		}
	}

	instance = &Whatsmiau{
		clients:            clients,
		container:          container,
		logger:             clientLog,
		repo:               repo,
		qrCache:            xsync.NewMap[string, string](),
		pairingCache:       xsync.NewMap[string, string](),
		instanceCache:      xsync.NewMap[string, models.Instance](),
		observerRunning:    xsync.NewMap[string, *whatsmeow.Client](),
		lockConnection:     xsync.NewMap[string, *sync.Mutex](),
		connectPhoneNumber: xsync.NewMap[string, string](),
		emitter:            make(chan emitter, env.Env.EmitterBufferSize),
		httpClient: &http.Client{
			Timeout: time.Second * 30, // TODO: load from env
		},
		fileStorage:      storage,
		handlerSemaphore: make(chan struct{}, env.Env.HandlerSemaphoreSize),
	}

	go instance.startEmitter()

	clients.Range(func(id string, client *whatsmeow.Client) bool {
		// Store.ID can become nil between Connect() above and this Range when the
		// server sends a logout/conflict event during the async connect.
		jid := ""
		if client.Store != nil && client.Store.ID != nil {
			jid = client.Store.ID.String()
		}
		zap.L().Info("stating event handler", zap.String("id", id), zap.String("jid", jid))
		client.AddEventHandler(instance.Handle(id))
		return true
	})

}

func (s *Whatsmiau) Connect(ctx context.Context, id string, phoneNumber string) (qrCode string, pairingCode string, err error) {
	// Tear down any in-flight QR/pair attempt when the client toggles between
	// QR-only and pairing-code mode. Once whatsmeow's QR channel is open, the
	// underlying session is "in QR mode"; calling PairPhone afterwards still
	// returns a code but WhatsApp later rejects it with "não foi possível
	// conectar". Starting fresh guarantees the session matches the method the
	// user is about to use.
	s.resetIfConnectMethodChanged(ctx, id, phoneNumber)

	client, err := s.generateClient(ctx, id)
	if err != nil {
		return "", "", err
	}
	if client == nil {
		return "", "", nil
	}

	if qr, ok := s.qrCache.Load(id); ok {
		pc, _ := s.pairingCache.Load(id)
		// A QR is already cached but the pairing code may be missing. This
		// happens when the observer was first started without a phoneNumber
		// (QR-only attempt) or when it is still mid-flight on PairPhone.
		// Request it explicitly so the caller does not race against an earlier
		// observer goroutine.
		if phoneNumber != "" && pc == "" {
			pc = s.ensurePairingCode(ctx, id, client, phoneNumber)
		}
		return qr, pc, nil
	}

	return s.observeAndQrCode(ctx, id, client, phoneNumber)
}

// resetIfConnectMethodChanged destroys an in-progress, not-yet-logged-in client
// when the caller switches between QR-only and pairing-code methods. whatsmeow's
// QR channel opens on the first connect and "locks" the session into QR mode —
// calling PairPhone later still returns a code but WhatsApp refuses the typed
// code. The only reliable way to switch methods is to discard the client and
// start over.
func (s *Whatsmiau) resetIfConnectMethodChanged(ctx context.Context, id, phoneNumber string) {
	lock, _ := s.lockConnection.LoadOrStore(id, &sync.Mutex{})
	lock.Lock()
	defer lock.Unlock()

	previous, hadPrevious := s.connectPhoneNumber.Load(id)
	s.connectPhoneNumber.Store(id, phoneNumber)

	if !hadPrevious || previous == phoneNumber {
		return
	}

	client, ok := s.clients.Load(id)
	if !ok {
		return
	}
	if client.IsLoggedIn() {
		return
	}

	zap.L().Info("reset pending connection due to connect method change",
		zap.String("id", id),
		zap.Bool("previous_had_number", previous != ""),
		zap.Bool("current_has_number", phoneNumber != ""),
	)

	// Closing the client also closes its QR channel, which unblocks the
	// observer goroutine and lets its deferred cleanup run.
	client.Disconnect()
	if err := s.deleteDeviceIfExists(ctx, client); err != nil {
		zap.L().Error("failed to delete device on method change", zap.String("id", id), zap.Error(err))
	}
	s.clients.Delete(id)
	s.qrCache.Delete(id)
	s.pairingCache.Delete(id)
	s.observerRunning.Delete(id)
}

// ensurePairingCode returns the cached pairing code, or requests a fresh one
// from WhatsApp and stores it. Safe to call concurrently with the observer
// goroutine — the per-instance lock plus the cache double-check prevent
// duplicate PairPhone RPCs.
func (s *Whatsmiau) ensurePairingCode(ctx context.Context, id string, client *whatsmeow.Client, phoneNumber string) string {
	lock, _ := s.lockConnection.LoadOrStore(id, &sync.Mutex{})
	lock.Lock()
	defer lock.Unlock()

	if pc, ok := s.pairingCache.Load(id); ok && pc != "" {
		return pc
	}

	code, err := client.PairPhone(ctx, phoneNumber, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		zap.L().Error("failed to request pairing code (fast-path)", zap.String("id", id), zap.Error(err))
		return ""
	}
	s.pairingCache.Store(id, code)
	return code
}

func (s *Whatsmiau) generateClient(ctx context.Context, id string) (*whatsmeow.Client, error) {
	lock, _ := s.lockConnection.LoadOrStore(id, &sync.Mutex{})
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

		if err := client.Connect(); err != nil {
			if client.IsLoggedIn() {
				return nil, nil
			}
			return nil, err
		}

		if client.IsLoggedIn() {
			return nil, nil
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

func (s *Whatsmiau) observeConnection(client *whatsmeow.Client, id string, phoneNumber string) {
	existingClient, loaded := s.observerRunning.LoadOrStore(id, client)
	if loaded {
		if existingClient == client {
			zap.L().Debug("observer connection already running for this client", zap.String("id", id))
			return
		}
		zap.L().Warn("replacing stale observer connection", zap.String("id", id))
		s.observerRunning.Store(id, client)
	}

	zap.L().Debug("starting observer connection", zap.String("id", id))
	defer func() {
		zap.L().Debug("stopping observer connection", zap.String("id", id))
		if currentClient, ok := s.observerRunning.Load(id); ok && currentClient == client {
			s.observerRunning.Delete(id)
			s.qrCache.Delete(id)
			s.pairingCache.Delete(id)
		}
	}()

	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute*2)
	qrChan, err := client.GetQRChannel(ctx)
	if err != nil {
		zap.L().Error("failed to observe QR Code", zap.Error(err))
		return
	}

	if instanceFound := s.getInstance(id); instanceFound != nil {
		configProxy(client, instanceFound.InstanceProxy)
	}
	if err := client.Connect(); err != nil {
		zap.L().Error("failed to connect connected device", zap.Error(err))
		return
	}

	zap.L().Debug("waiting for QR channel event", zap.String("id", id))
	emittedConnecting := false
	pairingRequested := false
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
				if !emittedConnecting {
					s.emitConnectionUpdate(id, "connecting", 0)
					emittedConnecting = true
				}
				s.qrCache.Store(id, evt.Code)

				if phoneNumber != "" && !pairingRequested {
					pairingRequested = true
					// Route through ensurePairingCode so that a concurrent
					// fast-path caller (Connect() after the QR is cached)
					// cannot trigger a second PairPhone RPC. WhatsApp
					// invalidates older codes when a new one is issued, so
					// duplicate requests lead to the frontend showing a
					// stale code that WhatsApp rejects with "could not
					// connect".
					s.ensurePairingCode(ctx, id, client, phoneNumber)
				}
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
				s.pairingCache.Delete(id)
				return
			}

			zap.L().Error("unknown event", zap.String("id", id), zap.Any("evt", evt))
		}
	}
}

func (s *Whatsmiau) observeAndQrCode(ctx context.Context, id string, client *whatsmeow.Client, phoneNumber string) (string, string, error) {
	ctx, c := context.WithTimeout(ctx, 15*time.Second)
	defer c()

	zap.L().Debug("starting observe and qr code", zap.String("id", id))
	go s.observeConnection(client, id, phoneNumber)

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			qrCode, ok := s.qrCache.Load(id)
			if ok && len(qrCode) > 0 {
				zap.L().Debug("got qr code from cache", zap.String("id", id))
				if phoneNumber != "" {
					// wait a bit more for pairing code to be generated
					pc, pcOk := s.pairingCache.Load(id)
					if pcOk {
						return qrCode, pc, nil
					}
					continue
				}
				return qrCode, "", nil
			}
		case <-ctx.Done():
			zap.L().Debug("observe and qr code context done", zap.String("id", id), zap.Error(ctx.Err()))
			// return whatever we have so far
			qr, _ := s.qrCache.Load(id)
			pc, _ := s.pairingCache.Load(id)
			if qr != "" {
				if phoneNumber != "" && pc == "" {
					return qr, "", ctx.Err()
				}
				return qr, pc, nil
			}
			return "", "", ctx.Err()
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
	client, ok := s.clients.Load(id)
	if !ok {
		zap.L().Warn("logout: client does not exist", zap.String("id", id))
		return nil
	}

	s.clients.Delete(id)
	return s.deleteDeviceIfExists(ctx, client)
}

func (s *Whatsmiau) Disconnect(id string) error {
	client, ok := s.clients.Load(id)
	if !ok {
		zap.L().Warn("failed to disconnect (device not loaded)", zap.String("id", id))
		return nil
	}

	client.Disconnect()
	s.qrCache.Delete(id)
	s.pairingCache.Delete(id)
	return nil
}

func (s *Whatsmiau) Restart(ctx context.Context, id string) error {
	lock, _ := s.lockConnection.LoadOrStore(id, &sync.Mutex{})
	lock.Lock()
	defer lock.Unlock()

	// Load fresh instance from Redis BEFORE destructive ops
	ctxInst, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	instances, err := s.repo.List(ctxInst, id)
	if err != nil {
		return fmt.Errorf("failed to load instance %s: %w", id, err)
	}
	if len(instances) == 0 {
		return fmt.Errorf("instance %s not found in redis", id)
	}
	instance := &instances[0]

	// Clean up existing client
	oldClient, hadClient := s.clients.Load(id)
	if hadClient {
		oldClient.RemoveEventHandlers()
		oldClient.Disconnect()
		s.clients.Delete(id)
	}

	// Clear caches
	s.qrCache.Delete(id)
	s.pairingCache.Delete(id)
	s.observerRunning.Delete(id)
	s.instanceCache.Delete(id)

	// Find device: old client's Store, or scan SQL store
	var device *store.Device
	if hadClient && oldClient.Store != nil && oldClient.Store.ID != nil {
		device = oldClient.Store
	} else if instance.RemoteJID != "" {
		jid, parseErr := types.ParseJID(instance.RemoteJID)
		if parseErr != nil {
			return fmt.Errorf("failed to parse RemoteJID %s: %w", instance.RemoteJID, parseErr)
		}
		device, err = s.container.GetDevice(ctx, jid)
		if err != nil {
			return fmt.Errorf("failed to get device for %s: %w", instance.RemoteJID, err)
		}
	}

	if device == nil {
		return fmt.Errorf("instance %s has no saved session — connect via QR code first", id)
	}

	client := whatsmeow.NewClient(device, s.logger)
	configProxy(client, instance.InstanceProxy)
	client.AddEventHandler(s.Handle(id))
	s.clients.Store(id, client)

	if err := client.Connect(); err != nil {
		zap.L().Error("restart: connect failed", zap.String("id", id), zap.Error(err))
		return err
	}

	zap.L().Info("restart: instance restarted successfully", zap.String("id", id))
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
	if !ok || client == nil || client.Store == nil || client.Store.LIDs == nil {
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
