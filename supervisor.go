// supervisor.go — cocuk surec (child process) supervizoru: `tray` komutu bu araciligiyla asil
// `masha-agent serve` surecini baslatir, cokerse (backoff ile) yeniden dogurur, Restart/Shutdown
// ile kontrol edilir. systray'e BAGLI DEGIL (test edilebilir, headless calisir).
package main

import (
	"log"
	"os/exec"
	"sync"
	"time"
)

// Supervisor — tek cocuk sureci gozetir. Spawn/Backoff test icin enjekte edilebilir.
type Supervisor struct {
	Spawn   func() *exec.Cmd // varsayilan: exec.Command(bin, args...)
	Backoff time.Duration    // cokme sonrasi yeniden-dogurma bekleme suresi (default 3s)

	mu           sync.Mutex
	cmd          *exec.Cmd
	shuttingDown bool
	generation   int // her Start/Restart ile artar; eski monitor goroutine'lerinin yanlislikla
	// yeni surec dogurmasini engeller (stale-goroutine korumasi).
}

// NewSupervisor — bin+args'tan varsayilan Spawn ile bir Supervisor kurar.
func NewSupervisor(bin string, args []string) *Supervisor {
	return &Supervisor{
		Spawn:   func() *exec.Cmd { return exec.Command(bin, args...) },
		Backoff: 3 * time.Second,
	}
}

// Start — cocugu dogurur ve gozetim goroutine'ini baslatir. Idempotent DEGIL (bir kez cagir);
// yeniden-baslatma icin Restart kullan.
func (s *Supervisor) Start() {
	s.mu.Lock()
	s.generation++
	gen := s.generation
	s.mu.Unlock()
	s.spawnAndWatch(gen)
}

// spawnAndWatch — sureci dogurur, cikisini bekler; shuttingDown degilse VE generation hala
// guncelse backoff sonrasi yeniden dogurur (eski nesil ise sessizce durur).
func (s *Supervisor) spawnAndWatch(gen int) {
	s.mu.Lock()
	if s.shuttingDown || gen != s.generation {
		s.mu.Unlock()
		return
	}
	cmd := s.Spawn()
	if err := cmd.Start(); err != nil {
		log.Printf("supervisor: cocuk surec baslatilamadi: %v", err)
		s.mu.Unlock()
		s.scheduleRespawn(gen)
		return
	}
	s.cmd = cmd
	s.mu.Unlock()

	log.Printf("supervisor: cocuk surec baslatildi (pid=%d)", cmd.Process.Pid)

	go func() {
		err := cmd.Wait()

		s.mu.Lock()
		shuttingDown := s.shuttingDown
		staleGen := gen != s.generation
		s.mu.Unlock()

		if shuttingDown {
			log.Printf("supervisor: cocuk surec kapandi (kapatma nedeniyle)")
			return
		}
		if staleGen {
			// Restart/Shutdown baska bir nesil baslatti/durdurdu — bu goroutine'in isi bitti.
			return
		}
		log.Printf("supervisor: cocuk surec cikti (%v) — %s sonra yeniden doguruluyor", err, s.backoff())
		s.scheduleRespawn(gen)
	}()
}

func (s *Supervisor) scheduleRespawn(gen int) {
	go func() {
		time.Sleep(s.backoff())
		s.spawnAndWatch(gen)
	}()
}

func (s *Supervisor) backoff() time.Duration {
	if s.Backoff > 0 {
		return s.Backoff
	}
	return 3 * time.Second
}

// Restart — mevcut cocugu oldurur; monitor goroutine'i (ayni nesil) yeniden dogurur. Cift-dogum
// yarisi generation sayaci ile onlenir: nesli artirip yeni gozetimi burada baslatiyoruz, eski
// nesil kendi Wait() donusunde staleGen gorup sessizce cikiyor.
func (s *Supervisor) Restart() {
	s.mu.Lock()
	if s.shuttingDown {
		s.mu.Unlock()
		return
	}
	old := s.cmd
	s.generation++
	gen := s.generation
	s.mu.Unlock()

	if old != nil && old.Process != nil {
		_ = old.Process.Kill()
	}
	log.Printf("supervisor: restart istendi")
	s.spawnAndWatch(gen)
}

// Shutdown — kapatma bayragini kaldirir, cocugu oldurur; bir daha dogurulmaz. Idempotent.
func (s *Supervisor) Shutdown() {
	s.mu.Lock()
	if s.shuttingDown {
		s.mu.Unlock()
		return
	}
	s.shuttingDown = true
	cmd := s.cmd
	s.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	log.Printf("supervisor: kapatildi")
}

// currentPID — testler icin: su an bilinen cocuk PID'i (yoksa 0).
func (s *Supervisor) currentPID() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil || s.cmd.Process == nil {
		return 0
	}
	return s.cmd.Process.Pid
}
