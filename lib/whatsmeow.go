package lib

import (
	"github.com/puzpuzpuz/xsync/v4"
	"github.com/verbeux-ai/whatsmiau/env"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"golang.org/x/net/context"
	"time"
)

type Whatsmiau struct {
	devices   *xsync.Map[*types.JID, *whatsmeow.Client]
	container *sqlstore.Container
	logger    waLog.Logger
}

var instance *Whatsmiau

func Get() *Whatsmiau {
	return instance
}

func LoadWhatsmiau(ctx context.Context, container *sqlstore.Container) {
	deviceStore, err := container.GetAllDevices(ctx)
	if err != nil {
		panic(err)
	}

	level := "INFO"
	if env.Env.DebugMode {
		level = "DEBUG"
	}

	devices := &xsync.Map[*types.JID, *whatsmeow.Client]{}
	clientLog := waLog.Stdout("Client", level, true)
	for _, device := range deviceStore {
		client := whatsmeow.NewClient(device, clientLog)
		devices.Store(client.Store.ID, client)
	}

	instance = &Whatsmiau{
		devices:   devices,
		container: container,
		logger:    clientLog,
	}
}

func (s *Whatsmiau) GetDevice(id *types.JID) *whatsmeow.Client {
	v, ok := s.devices.Load(id)
	if !ok {
		return nil
	}

	return v
}

func (s *Whatsmiau) AddDevice() (*whatsmeow.Client, error) {
	device := s.container.NewDevice()

	client := whatsmeow.NewClient(device, s.logger)
	ctx := context.Background()

	qrChan, err := client.GetQRChannel(ctx)
	if err != nil {
		return nil, err
	}

	go func() {
		tick := time.NewTicker(time.Minute * 30)
		defer tick.Stop()

		for event := range qrChan {
			if event.Code != "code" {
				time.Sleep(5 * time.Second)
				jid := device.GetJID()
				s.devices.Store(&jid, client)
			}
		}
	}()

	return client, nil
}
