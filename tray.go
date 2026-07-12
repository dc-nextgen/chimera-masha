// tray.go — masaustu "hep-acik" yuz: sistem tepsisi ikonu + menu (Ac · Restart · Ayarlar · Cikis),
// asil `masha-agent serve` surecini COCUK SUREC olarak gozetir (supervisor.go). `service` komutundan
// (kardianos, headless) AYRI — burasi login-item/GUI-oturum yuzu, orasi reboot-proof OS servisi.
package main

import (
	_ "embed"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"runtime"

	"fyne.io/systray"

	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/config"
)

//go:embed tray/icon.png
var trayIconBytes []byte

var traySupervisor *Supervisor

// runTray — `masha-agent tray` giris noktasi. systray.Run ANA goroutine'de cagrilmali (bloklar);
// main() zaten dogrudan (goroutine ACMADAN) buraya dispatch ettigi icin bu kosul saglanir.
func runTray() {
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		fmt.Fprintln(os.Stderr, "masaustu oturumu bulunamadi — sunucu/headless kurulumda 'masha-agent service install' kullanin")
		os.Exit(1)
	}

	base := trayBaseURL()

	bin, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: kendi yolum bulunamadi: %v\n", err)
		os.Exit(1)
	}
	traySupervisor = NewSupervisor(bin, []string{"serve"})
	traySupervisor.Start()

	systray.Run(func() { onTrayReady(base) }, onTrayExit)
}

// trayBaseURL — yerel web yuzun tarayicida acilacak temel URL'i. cfg.WebAddr host'u 0.0.0.0/::/bos
// ise (her arayuze bagli) yerel acilis icin 127.0.0.1'e degistirilir (port korunur).
func trayBaseURL() string {
	cfg, _ := config.Load()
	scheme := "http"
	if cfg.WebTLS {
		scheme = "https"
	}
	host, port, err := net.SplitHostPort(cfg.WebAddr)
	if err != nil {
		host, port = cfg.WebAddr, ""
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	if port == "" {
		return fmt.Sprintf("%s://%s", scheme, host)
	}
	return fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(host, port))
}

func onTrayReady(base string) {
	systray.SetIcon(trayIconBytes)
	systray.SetTitle("")
	systray.SetTooltip("Masha Agent")

	mOpen := systray.AddMenuItem("Aç", "Paneli tarayıcıda aç")
	mRestart := systray.AddMenuItem("Restart", "Ajanı yeniden başlat")
	mSettings := systray.AddMenuItem("Ayarlar", "Ayarları aç")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Çıkış", "Masha'yı kapat")

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				openURL(base)
			case <-mRestart.ClickedCh:
				traySupervisor.Restart()
			case <-mSettings.ClickedCh:
				u, err := url.JoinPath(base, "ayarlar")
				if err != nil {
					u = base + "/ayarlar"
				}
				openURL(u)
			case <-mQuit.ClickedCh:
				traySupervisor.Shutdown()
				systray.Quit()
				return
			}
		}
	}()
}

// onTrayExit — systray.Quit() cagrildiginda VEYA OS tepsiyi kapattiginda cagrilir. Shutdown
// idempotent oldugu icin mQuit yolundan zaten cagrilmis olsa da guvenle tekrar cagrilabilir.
func onTrayExit() {
	if traySupervisor != nil {
		traySupervisor.Shutdown()
	}
}

// openURL — platforma gore varsayilan tarayiciyi/handler'i acar. Hata sessizce yutulur (tepsi
// menusunden acilan bir kolayliktir, kritik yol degil).
func openURL(u string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	_ = cmd.Start()
}
