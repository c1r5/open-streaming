package dlna

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	dms "github.com/anacrolix/dms/dlna/dms"
	"github.com/c1r5/open-streaming/src/modules/dlna/application"
	"github.com/c1r5/open-streaming/src/shared/config"
	"gorm.io/gorm"
)

type Module struct {
	server *dms.Server
}

func New(db *gorm.DB, cfg *config.Config) (*Module, error) {
	if !cfg.DLNA.Enabled {
		return nil, nil
	}

	httpAddr := httpServerAddr(cfg)
	svc := application.NewService(db, httpAddr)

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.DLNA.Port))
	if err != nil {
		return nil, fmt.Errorf("dlna: listen :%d: %w", cfg.DLNA.Port, err)
	}

	// RootObjectPath must be a valid directory; use a temp dir since we only use callbacks.
	tmpDir, err := os.MkdirTemp("", "dlna-root-*")
	if err != nil {
		ln.Close()
		return nil, fmt.Errorf("dlna: temp dir: %w", err)
	}

	srv := &dms.Server{
		HTTPConn:               ln,
		FriendlyName:           cfg.DLNA.FriendlyName,
		RootObjectPath:         tmpDir,
		OnBrowseDirectChildren: svc.BrowseDirectChildren,
		OnBrowseMetadata:       svc.BrowseMetadata,
		NoTranscode:            true,
		NoProbe:                true,
		NotifyInterval:         30 * time.Second,
	}

	if cfg.DLNA.Interface != "" {
		iface, err := net.InterfaceByName(cfg.DLNA.Interface)
		if err != nil {
			ln.Close()
			return nil, fmt.Errorf("dlna: interface %q: %w", cfg.DLNA.Interface, err)
		}
		srv.Interfaces = []net.Interface{*iface}
	}

	return &Module{server: srv}, nil
}

func (m *Module) Start() error {
	if err := m.server.Init(); err != nil {
		return fmt.Errorf("dlna: init: %w", err)
	}

	go func() {
		if err := m.server.Run(); err != nil {
			log.Printf("dlna: server stopped: %v", err)
		}
	}()

	log.Printf("DLNA server listening on %s", m.server.HTTPConn.Addr())
	return nil
}

func (m *Module) Stop() {
	if m == nil || m.server == nil {
		return
	}
	if err := m.server.Close(); err != nil {
		log.Printf("dlna: close: %v", err)
	}
	// Clean up temp dir
	os.RemoveAll(m.server.RootObjectPath)
}

func httpServerAddr(cfg *config.Config) string {
	host := preferredIP()
	return fmt.Sprintf("http://%s:%s", host, cfg.Server.Port)
}

func preferredIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}
