package workload

import "testing"

func BenchmarkJSON(b *testing.B) {
	w, err := New("json", []byte("json_file: ../data/small.json"))
	if err != nil {
		b.Fatal(err)
	} else if err := w.Setup(); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Run()
	}
}
