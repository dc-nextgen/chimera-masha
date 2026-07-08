// Package tunnel — bulut edge frps'e DISARI-arayan reverse-tunnel.
//
// §18 Faz 1: frpc SIDECAR (mevcut frpc.toml + statik frpc ikilisi; "embed"e dusememe fallback'i,
// docs §18). frpc tek statik Go ikilisi = "runtime" degil, yalniz bir yardimci ikili. FrpcConfig
// bos ise tunel KAPALI (yalniz-yerel gelistirme/test). mTLS/token frpc.toml'da (kanal 3-faktor).
package tunnel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

type Tunnel interface {
	Start(ctx context.Context) error // blocking degil; arka-plan surec baslatir
	Stop() error
}

// SidecarFrpc — frpc ikilisini `-c <config>` ile calistirir, cikarsa yeniden baslatmayi
// frpc'nin kendi loginFailExit=false + reconnect'ine birakir (unattended, §16.3).
type SidecarFrpc struct {
	Bin    string
	Config string
	cmd    *exec.Cmd
}

func NewSidecar(bin, config string) *SidecarFrpc {
	return &SidecarFrpc{Bin: bin, Config: config}
}

func (s *SidecarFrpc) Start(ctx context.Context) error {
	if s.Config == "" {
		return nil // tunel KAPALI (yalniz-yerel mod)
	}
	if _, err := os.Stat(s.Config); err != nil {
		return fmt.Errorf("frpc config bulunamadi: %s", s.Config)
	}
	s.cmd = exec.CommandContext(ctx, s.Bin, "-c", s.Config)
	s.cmd.Stdout = os.Stdout
	s.cmd.Stderr = os.Stderr
	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("frpc baslatilamadi (%s): %w", s.Bin, err)
	}
	return nil
}

func (s *SidecarFrpc) Stop() error {
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Kill()
	}
	return nil
}
