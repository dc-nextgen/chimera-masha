// embedded.go — frpc SÜREÇ-İÇİNDE (embedded) istemci. §18: OrbStack/Docker uykuya dalınca
// sidecar frpc konteyneri de uyur → tünel kopar. Embedded modda frp client kütüphanesi
// (github.com/fatedier/frp) doğrudan bu ikilinin içinde çalışır — Docker/harici frpc
// bağımlılığı YOK. Aynı frpc.toml'u frp'nin KENDİ yükleyicisiyle okur (assemble/render
// pipeline'ı değişmez); glue kod cmd/frpc/sub/root.go'daki resmi akışın birebir aynısı
// (go doc + kaynak ile doğrulandı, frp v0.70.0).
package tunnel

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/fatedier/frp/client"
	"github.com/fatedier/frp/client/proxy"
	frpconfig "github.com/fatedier/frp/pkg/config"
	"github.com/fatedier/frp/pkg/config/source"
	"github.com/fatedier/frp/pkg/config/v1/validation"
	"github.com/fatedier/frp/pkg/policy/security"
	frplog "github.com/fatedier/frp/pkg/util/log"
)

// conflictMsg — sidecar VE embedded ortak "aynı-slot çatışması" Türkçe mesajı (§ ayni-bundle
// iki-makine catismasi). Tek kaynak: her iki implementasyon da bunu kullanir.
const conflictMsg = "Bu bağlantı slotu zaten BAŞKA bir makinede aktif — edge (frps) ikinci ajanı reddetti. Aynı kurulum paketi iki makinede aynı anda çalışamaz. Kasıtlı ikinci veri kaynağıysa Pota'dan AYRI bir bağlantı ekleyin (ayrı slot)."

// EmbeddedFrpc — frp client.Service'i dogrudan bu surecte calistirir (Docker/harici frpc YOK).
// Config bos ise (yalniz-yerel mod) Start no-op, Status "off" — SidecarFrpc ile AYNI davranis.
type EmbeddedFrpc struct {
	Config string

	mu        sync.Mutex
	svr       *client.Service
	proxyName string // status sorgusu icin ilk proxy adi
	started   bool
	loadErr   error // config yuklenemedi/gecersizse Start once bunu dondurur, Status "error" gosterir
}

func NewEmbedded(config string) *EmbeddedFrpc {
	return &EmbeddedFrpc{Config: config}
}

// mapPhase — frp'nin proxy.WorkingStatus.Phase (+ Err) degerini bizim durum sozlesmemize
// (off/connecting/connected/conflict/error) cevirir. Saf fonksiyon — canli frps'siz testlenebilir.
func mapPhase(phase, errMsg string) (state, msg string) {
	switch phase {
	case proxy.ProxyPhaseRunning:
		return "connected", ""
	case proxy.ProxyPhaseNew, proxy.ProxyPhaseWaitStart:
		return "connecting", ""
	case proxy.ProxyPhaseStartErr, proxy.ProxyPhaseCheckFailed:
		if strings.Contains(strings.ToLower(errMsg), "already") {
			return "conflict", conflictMsg
		}
		return "error", errMsg
	case proxy.ProxyPhaseClosed:
		return "off", ""
	default:
		return "connecting", ""
	}
}

func (e *EmbeddedFrpc) Status() (state, msg string) {
	if e.Config == "" {
		return "off", ""
	}
	e.mu.Lock()
	svr := e.svr
	name := e.proxyName
	started := e.started
	loadErr := e.loadErr
	e.mu.Unlock()

	if loadErr != nil {
		return "error", loadErr.Error()
	}
	if !started || svr == nil {
		return "connecting", ""
	}
	exp := svr.StatusExporter()
	if exp == nil {
		return "connecting", ""
	}
	ws, ok := exp.GetProxyStatus(name)
	if !ok || ws == nil {
		return "connecting", ""
	}
	return mapPhase(ws.Phase, ws.Err)
}

// Start — frpc.toml'u frp'nin KENDI yukleyicisiyle okur, dogrular, client.Service olusturur
// ve Run(ctx)'i arka-plan goroutine'de baslatir (Run BLOKE olur — bu yuzden bloke ETMEYEN
// Start sozlesmesini korumak icin goroutine'e alinir, sidecar Start'la ayni davranis).
func (e *EmbeddedFrpc) Start(ctx context.Context) error {
	if e.Config == "" {
		return nil // tunel KAPALI (yalniz-yerel mod)
	}

	result, err := frpconfig.LoadClientConfigResult(e.Config, true)
	if err != nil {
		e.mu.Lock()
		e.loadErr = err
		e.mu.Unlock()
		return fmt.Errorf("frpc.toml yuklenemedi: %w", err)
	}

	unsafeFeatures := security.NewUnsafeFeatures(nil)

	cfgSource := source.NewConfigSource()
	if err := cfgSource.ReplaceAll(result.Proxies, result.Visitors); err != nil {
		e.mu.Lock()
		e.loadErr = err
		e.mu.Unlock()
		return fmt.Errorf("frpc config kaynagi kurulamadi: %w", err)
	}
	aggregator := source.NewAggregator(cfgSource)

	proxyCfgs, visitorCfgs, err := aggregator.Load()
	if err != nil {
		e.mu.Lock()
		e.loadErr = err
		e.mu.Unlock()
		return fmt.Errorf("frpc config yuklenemedi (aggregator): %w", err)
	}
	proxyCfgs, visitorCfgs = frpconfig.FilterClientConfigurers(result.Common, proxyCfgs, visitorCfgs)
	proxyCfgs = frpconfig.CompleteProxyConfigurers(proxyCfgs)
	visitorCfgs = frpconfig.CompleteVisitorConfigurers(visitorCfgs)

	if warning, err := validation.ValidateAllClientConfig(result.Common, proxyCfgs, visitorCfgs, unsafeFeatures); err != nil {
		e.mu.Lock()
		e.loadErr = err
		e.mu.Unlock()
		return fmt.Errorf("frpc config gecersiz: %w", err)
	} else if warning != nil {
		fmt.Println("UYARI (frpc embed):", warning)
	}

	var firstProxyName string
	if len(proxyCfgs) > 0 {
		firstProxyName = proxyCfgs[0].GetBaseConfig().Name
	}

	frplog.InitLogger(result.Common.Log.To, result.Common.Log.Level, int(result.Common.Log.MaxDays), result.Common.Log.DisablePrintColor)

	svr, err := client.NewService(client.ServiceOptions{
		Common:                 result.Common,
		ConfigSourceAggregator: aggregator,
		UnsafeFeatures:         unsafeFeatures,
		ConfigFilePath:         e.Config,
	})
	if err != nil {
		e.mu.Lock()
		e.loadErr = err
		e.mu.Unlock()
		return fmt.Errorf("frp client.Service olusturulamadi: %w", err)
	}

	e.mu.Lock()
	e.svr = svr
	e.proxyName = firstProxyName
	e.started = true
	e.loadErr = nil
	e.mu.Unlock()

	go func() {
		if err := svr.Run(ctx); err != nil {
			// Run ctx iptalinde de err dondurebilir (normal kapanma) — sadece logla, agent CIKMAZ.
			fmt.Println("tunel (embed frpc) durdu:", err)
		}
	}()

	return nil
}

func (e *EmbeddedFrpc) Stop() error {
	e.mu.Lock()
	svr := e.svr
	e.mu.Unlock()
	if svr != nil {
		svr.Close()
	}
	return nil
}
