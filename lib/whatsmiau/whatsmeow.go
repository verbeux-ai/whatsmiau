package whatsmiau

import (
	"fmt"
	"net/http"
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
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type Whatsmiau struct {
	clients          *xsync.Map[string, *whatsmeow.Client]
	container        *sqlstore.Container
	logger           waLog.Logger
	repo             interfaces.InstanceRepository
	qrCache          *xsync.Map[string, string]
	observerRunning  *xsync.Map[string, bool]
	instanceCache    *xsync.Map[string, models.Instance]
	emitter          chan emitter
	httpClient       *http.Client
	fileStorage      interfaces.Storage
	handlerSemaphore chan struct{}
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
		zap.L().Fatal("Failed to list instances", zap.Error(err))
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
		if err := client.Connect(); err != nil {
			zap.L().Error("failed to connect connected device", zap.Error(err), zap.String("jid", client.Store.ID.String()))
		}

		if client.Store.ID == nil {
			_ = client.Logout(context.Background())
			client.Disconnect()
			continue
		}

		instanceFound, ok := instanceByRemoteJid[client.Store.ID.String()]
		if ok {
			clients.Store(instanceFound.ID, client)
		} else {
			_ = client.Logout(context.Background())
			client.Disconnect()
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
		clients:         clients,
		container:       container,
		logger:          clientLog,
		repo:            repo,
		qrCache:         xsync.NewMap[string, string](),
		instanceCache:   xsync.NewMap[string, models.Instance](),
		observerRunning: xsync.NewMap[string, bool](),
		emitter:         make(chan emitter, env.Env.EmitterBufferSize),
		httpClient: &http.Client{
			Timeout: time.Second * 30, // TODO: load from env
		},
		fileStorage:      storage,
		handlerSemaphore: make(chan struct{}, env.Env.HandlerSemaphoreSize),
	}

	go instance.startEmitter()

	clients.Range(func(id string, client *whatsmeow.Client) bool {
		zap.L().Info("stating event handler", zap.String("jid", client.Store.ID.String()))
		client.AddEventHandler(instance.Handle(id))
		return true
	})

}

func (s *Whatsmiau) Connect(ctx context.Context, id string) (string, error) {
	client, ok := s.clients.Load(id)
	if !ok {
		device := s.container.NewDevice()
		device.Platform = "Verboo 👻"
		client = whatsmeow.NewClient(device, s.logger)
		s.clients.Store(id, client)
	}

	if client.IsLoggedIn() {
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

func (s *Whatsmiau) observeConnection(client *whatsmeow.Client, id string) (chan string, error) {
	qrStringChan := make(chan string)

	go func() {
		if _, ok := s.observerRunning.Load(id); ok {
			ticker := time.NewTicker(1 * time.Second)
			select {
			case <-ticker.C:
				qrCode, ok := s.qrCache.Load(id)
				if ok && len(qrCode) > 0 {
					qrStringChan <- qrCode
					break
				}
			case <-time.After(15 * time.Second):
				qrStringChan <- ""
				break
			}
			return
		}

		s.observerRunning.Store(id, true)
		defer s.observerRunning.Delete(id)

		qrChan, err := client.GetQRChannel(context.Background())
		if err != nil {
			zap.L().Error("failed to observe QR Code", zap.Error(err))
			qrStringChan <- ""
			return
		}

		if !client.IsConnected() {
			if err := client.Connect(); err != nil {
				zap.L().Error("failed to connect", zap.Error(err))
				qrStringChan <- ""
				return
			}
		}

		canStop := false
		for !canStop {
			select {
			case <-time.After(2 * time.Minute): // QR code expiration
				_ = client.Logout(context.Background())
				client.Disconnect()
				s.clients.Delete(id)
				s.qrCache.Delete(id)
				zap.L().Info("QR code expired, disconnected client", zap.String("id", id))
				canStop = true
			case evt, ok := <-qrChan:
				if !ok {
					zap.L().Warn("QR channel closed while handling post-qr events", zap.String("id", id))
					qrStringChan <- ""
					canStop = true
					s.clients.Delete(id)
					s.qrCache.Delete(id)
				}
				if evt.Event == "code" {
					qrStringChan <- evt.Code
					s.qrCache.Store(id, evt.Code)
				} else {
					canStop = true
					zap.L().Info("device connected successfully", zap.String("id", id))
					if client.Store.ID == nil {
						s.clients.Delete(id)
						s.qrCache.Delete(id)
						zap.L().Error("jid is nil after login", zap.String("id", id))
					} else {
						client.RemoveEventHandlers()
						client.AddEventHandler(s.Handle(id))
						if err := s.repo.Update(context.Background(), id, &models.Instance{
							RemoteJID: client.Store.ID.String(),
						}); err != nil {
							zap.L().Error("failed to update instance after login", zap.Error(err))
						}
					}
				}
			}
		}
	}()

	return qrStringChan, nil
}

func (s *Whatsmiau) observeAndQrCode(ctx context.Context, id string, client *whatsmeow.Client) (string, error) {
	ctx, c := context.WithTimeout(ctx, 15*time.Second)
	defer c()

	qrStringChan, err := s.observeConnection(client, id)
	if err != nil {
		zap.L().Error("failed to observe QR Code", zap.Error(err))
		return "", err
	}

	var qrCode string
	select {
	case qr, ok := <-qrStringChan:
		if !ok {
			return "", fmt.Errorf("qr channel closed unexpectedly")
		}
		qrCode = qr
	case <-ctx.Done():
		zap.L().Warn("context canceled", zap.String("id", id))
		return "", ctx.Err()
	}

	if qrCode == "" {
		return "", fmt.Errorf("qr code is empty")
	}

	s.qrCache.Store(id, qrCode)

	return qrCode, nil
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

	return client.Logout(ctx)
}

func (s *Whatsmiau) Disconnect(id string) error {
	client, ok := s.clients.Load(id)
	if !ok {
		zap.L().Warn("failed to disconnect (device not loaded)", zap.String("id", id))
		return nil
	}

	client.Disconnect()

	s.clients.Delete(id)
	s.qrCache.Delete(id)
	return nil
}
