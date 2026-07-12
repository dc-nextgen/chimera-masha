// service.go — kardianos ile reboot-proof native OS servisi (launchd/systemd/Windows Service).
//
// `serve` foreground davranisi (Ctrl-C, signal.NotifyContext) DEGISMEDEN kalir; bu dosya AYRI bir
// yol ekler: `masha-agent service install|uninstall|start|stop|restart|status`. Kurulu servis, OS
// service manager'i tarafindan `masha-agent serve` calistirilarak baslatilir (Arguments=["serve"]),
// coker/reboot olursa OS yeniden baslatir (systemd Restart=always, launchd KeepAlive).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kardianos/service"
)

// mashaProgram — service.Interface: Start NON-BLOCKING olmali (kardianos gereksinimi), asil is
// goroutine'de calisir; Stop cagrildiginda graceful kapanis icin process'e SIGTERM gonderir
// (runServe zaten signal.NotifyContext ile SIGTERM'i dinliyor → tek kapanis yolu, kod tekrari yok).
type mashaProgram struct {
	exit chan struct{}
}

func (p *mashaProgram) Start(s service.Service) error {
	go p.run()
	return nil
}

func (p *mashaProgram) run() {
	runServe()
	close(p.exit)
}

func (p *mashaProgram) Stop(s service.Service) error {
	// runServe kendi signal.NotifyContext'i ile SIGINT/SIGTERM dinliyor; servis modunda OS bize
	// SIGTERM'i zaten iletir (kardianos ayri bir sinyal katmani eklemez) → burada ek is YOK,
	// yalnizca kardianos'un beklemesi icin run() bitene kadar bekleriz (kisa timeout ile).
	select {
	case <-p.exit:
	default:
	}
	return nil
}

// serviceUserMode — MASHA_SERVICE_USER truthy ise per-user (macOS LaunchAgent / systemd --user)
// servis kurulur. Neden: macOS'ta sistem LaunchDaemon HEADLESS oturumda calisir (masaustu/GUI
// gerektiren hicbir sey yok burada ama operator genelde kendi login-session'inda test eder);
// varsayilan (false) = sistem servisi (LaunchDaemon/systemd system unit), coğu on-prem kurulum
// icin dogru (reboot'ta kullanici girisi beklemeden ayaga kalkar).
func serviceUserMode() bool {
	v := strings.TrimSpace(os.Getenv("MASHA_SERVICE_USER"))
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// captureMashaEnv — mevcut process ortamindan MASHA_* (ve config.go'nun envOr fallback'i icin
// ciplak ERPNEXT_*) degiskenlerini toplar. `service install` zamaninda kardianos.Config.EnvVars'a
// yazilir → uretilen launchd plist / systemd unit dosyasina gomulur. GEREKLI: agent TAMAMEN env'den
// yapilandirilir (config.go); servis olarak calisirken operatorun shell'inden kalitim YOKTUR,
// EnvVars olmadan servis auth/DB/manifest'siz acilip hemen fail-closed olur (RequireServe). Bu
// yuzden sessizce atlanmaz — EnvVars her zaman doldurulur. ERPNEXT_* de kapsanir cunku config.go
// envOr("MASHA_ERPNEXT_URL","ERPNEXT_URL") ile stack/.env ciplak adlarina duser.
func captureMashaEnv() map[string]string {
	out := make(map[string]string)
	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if strings.HasPrefix(k, "MASHA_") || strings.HasPrefix(k, "ERPNEXT_") {
			out[k] = v
		}
	}
	return out
}

// buildServiceConfig — kardianos service.Config'i env'den kurar (test edilebilir, kardianos'a
// bagli olmayan saf mantik). workDir = servisin CALISMA DIZINI: bundle'da MASHA_FRPC_CONFIG=./frpc.toml,
// MASHA_MANIFEST=./..., certs/ hep GORELI → servis / dizininden calisirsa bunlari BULAMAZ. install
// anindaki binary dizini (bundle) WorkingDirectory olarak set edilir → goreli yollar cozulur.
func buildServiceConfig(env map[string]string, userMode bool, workDir string) *service.Config {
	cfg := &service.Config{
		Name:             "masha-agent",
		DisplayName:      "Masha Agent",
		Description:      "Masha on-prem connector ajani (Chimera AI)",
		Arguments:        []string{"serve"},
		EnvVars:          env,
		WorkingDirectory: workDir,
		Option: service.KeyValue{
			"UserService": userMode,
			"Restart":     "always", // systemd: Restart=always
			"KeepAlive":   true,     // launchd: KeepAlive=true (coktugunde yeniden baslatir)
			"RunAtLoad":   true,     // launchd: acilista baslat (reboot-proof)
		},
	}
	return cfg
}

// serviceWorkDir — kurulan servisin calisma dizini = binary'nin bulundugu dizin (bundle). os.Executable
// symlink cozer; hata olursa cwd'ye duser (install.sh zaten `cd "$(dirname "$0")"` yapar).
func serviceWorkDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return ""
}

// newService — os.Executable() ile mevcut binary yolunu kullanir (kurulan servis HER ZAMAN
// `masha-agent serve` calistirir; Arguments bunu saglar).
func newService() (service.Service, error) {
	userMode := serviceUserMode()
	svcCfg := buildServiceConfig(captureMashaEnv(), userMode, serviceWorkDir())
	prg := &mashaProgram{exit: make(chan struct{})}
	return service.New(prg, svcCfg)
}

// runServiceCmd — `masha-agent service <action>` dispatch. action: install|uninstall|start|stop|
// restart|status. status kardianos'a degil dogrudan service.Status()'a sorar, Turkce insan-okur
// satir basar.
func runServiceCmd(action string) {
	svc, err := newService()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: servis olusturulamadi: %v\n", err)
		os.Exit(1)
	}

	switch action {
	case "status":
		st, err := svc.Status()
		if err != nil {
			// kardianos: servis kurulu degilse service.ErrNotInstalled doner.
			fmt.Println("servis: kurulu değil")
			return
		}
		switch st {
		case service.StatusRunning:
			fmt.Println("servis: çalışıyor")
		case service.StatusStopped:
			fmt.Println("servis: durdu")
		default:
			fmt.Println("servis: bilinmiyor")
		}
	case "install", "uninstall", "start", "stop", "restart":
		if err := service.Control(svc, action); err != nil {
			fmt.Fprintf(os.Stderr, "FATAL: servis %s başarısız: %v\n", action, err)
			os.Exit(1)
		}
		fmt.Printf("servis %s: tamam\n", action)
		if action == "install" {
			mode := "sistem"
			if serviceUserMode() {
				mode = "kullanıcı"
			}
			fmt.Printf("  (%s servisi kuruldu — reboot'ta otomatik başlar, çöküşte yeniden başlar)\n", mode)
		}
	default:
		fmt.Fprintf(os.Stderr, "bilinmeyen servis eylemi: %s (install|uninstall|start|stop|restart|status)\n", action)
		os.Exit(2)
	}
}
