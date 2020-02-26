package ztream

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

var bench = []struct {
	name string
	size int
}{
	{"1k", 1024},
	{"8k", 1024 * 8},
	{"1M", 1024 * 1024},
	{"10M", 10 * 1024 * 1024},
}

func BenchmarkSequential(b *testing.B) {
	fn := file(b)
	defer clean()

	// w / wo sync at every

	for _, be := range bench {
		buf := data(be.size, false)
		b.Run("Append"+be.name, func(b *testing.B) {
			no := 0
			runtime.GC()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				os.Remove(fn)
				s, err := Create(fn, Options{})
				for err == nil {
					_, err = s.Append("a", buf)
					no++
				}
				if err != ErrStreamFull {
					b.Error(err)
				}
				if err := s.Sync(); err != nil {
					b.Error(err)
				}
				if err := s.Close(); err != nil {
					b.Error(err)
				}
				no--
			}
			b.SetBytes(int64(len(buf) * no / b.N))
		})
		b.Run("AppendSync"+be.name, func(b *testing.B) {
			no := 0
			runtime.GC()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				os.Remove(fn)
				s, err := Create(fn, Options{})
				for err == nil {
					_, err = s.Append("a", buf)
					if err != nil {
						if err := s.Sync(); err != nil {
							b.Error(err)
						}
					}
					no++
				}
				if err != ErrStreamFull {
					b.Error(err)
				}
				if err := s.Close(); err != nil {
					b.Error(err)
				}
				no--
			}
			b.SetBytes(int64(len(buf) * no / b.N))
		})
		b.Run("Read"+be.name, func(b *testing.B) {
			s, err := Open(fn, Options{})
			if err != nil {
				b.Error(err)
			}
			cts, err := s.Contents()
			if err != nil || len(cts) < 1 {
				b.Error(err)
			}
			s.Close()

			b.SetBytes(int64(len(buf) * len(cts)))
			runtime.GC()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				clearDiskCache(fn)
				b.StartTimer()
				s, _ := Open(fn, Options{})
				for _, e := range cts {
					err := s.Read(e, buf)
					if err != nil {
						b.Error(err)
					}
				}
				s.Close()
			}
		})
	}
}

func clearDiskCache(fn string) {
	if runtime.GOOS == "darwin" {
		// note that this requires sudo...
		cmd := exec.Command("purge")
		err := cmd.Run()
		if err != nil {
			println(err.Error())
			panic(err.Error())
		}
	} else {
		panic("buffer clearning not implemented on this os")
	}
}
