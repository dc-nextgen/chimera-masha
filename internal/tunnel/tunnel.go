// Package tunnel — bulut edge frps'e DISARI-arayan reverse-tunnel.
//
// §18 Faz 1: frpc SIDECAR (mevcut frpc.toml + statik frpc ikilisi; "embed"e dusememe fallback'i,
// docs §18). frpc tek statik Go ikilisi = "runtime" degil, yalniz bir yardimci ikili. FrpcConfig
// bos ise tunel KAPALI (yalniz-yerel gelistirme/test). mTLS/token frpc.toml'da (kanal 3-faktor).
package tunnel

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
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

	// mu + state/msg — frpc cikti akisindan cikarilan tunel DURUMU (§ ayni-bundle iki-makine
	// catismasi). healthz bunu okur; agent CIKMAZ (yerel yuz ayakta kalir, kullanici goru).
	mu    sync.Mutex
	state string
	msg   string
}

func NewSidecar(bin, config string) *SidecarFrpc {
	return &SidecarFrpc{Bin: bin, Config: config}
}

// Status — tunel DURUMU (healthz icin). Config bos ise tunel hic acilmamis ("off").
// Aksi halde son bilinen durum ("connecting" varsayilan, henuz frpc satiri gelmediyse).
func (s *SidecarFrpc) Status() (state, msg string) {
	if s.Config == "" {
		return "off", ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.state
	if st == "" {
		st = "connecting"
	}
	return st, s.msg
}

// setState — durumu gunceller. "conflict"e GECISTE (once baska durumdaysa) bir kere belirgin
// UYARI loglar — her satirda tekrar etmesin diye yalniz gecis anini yakalar.
func (s *SidecarFrpc) setState(state, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state != s.state && state == "conflict" {
		log.Printf("UYARI: tunel CATISMASI tespit edildi — %s", msg)
	}
	s.state, s.msg = state, msg
}

// inspectLine — frpc'nin bir satirini catisma/baglanti sinyali icin tarar (testlenebilir, ayri).
// "already exists/used/in use" = ayni proxy adi baska makinede aktif (ayni bundle iki-makine).
// Sonraki "start proxy success" catismayi TEMIZLER (mesru ayni-makine restart yanlis-pozitif olmasin).
func (s *SidecarFrpc) inspectLine(line string) {
	l := strings.ToLower(line)
	switch {
	case strings.Contains(l, "already exists"), strings.Contains(l, "already used"), strings.Contains(l, "already in use"):
		s.setState("conflict", conflictMsg)
	case strings.Contains(l, "start proxy success"):
		s.setState("connected", "")
	}
}

// scan — bir frpc cikti akisini (stdout/stderr) satir satir okur: once oldugu gibi ILET (mevcut
// loglama davranisi korunur), sonra catisma/baglanti sinyali icin tara.
func (s *SidecarFrpc) scan(r io.Reader) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		fmt.Fprintln(os.Stdout, line)
		s.inspectLine(line)
	}
}

func (s *SidecarFrpc) Start(ctx context.Context) error {
	if s.Config == "" {
		return nil // tunel KAPALI (yalniz-yerel mod)
	}
	if _, err := os.Stat(s.Config); err != nil {
		return fmt.Errorf("frpc config bulunamadi: %s", s.Config)
	}
	s.cmd = exec.CommandContext(ctx, s.Bin, "-c", s.Config)
	stdout, outErr := s.cmd.StdoutPipe()
	stderr, errErr := s.cmd.StderrPipe()
	if outErr != nil || errErr != nil {
		// pipe kurulamadi — dogrudan yaz (eski davranis), tunel yine de baslasin.
		s.cmd.Stdout = os.Stdout
		s.cmd.Stderr = os.Stderr
	}
	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("frpc baslatilamadi (%s): %w", s.Bin, err)
	}
	if outErr == nil && errErr == nil {
		go s.scan(stdout)
		go s.scan(stderr)
	}
	return nil
}

func (s *SidecarFrpc) Stop() error {
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Kill()
	}
	return nil
}
