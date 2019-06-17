package rsm

import (
	"math/rand"
	"sort"
	"sync"
	"testing"
)

func TestCommittedQueue(t *testing.T) {
	var q CommittedQueue

	for i := 0; i < 10; i++ {
		id := uint32(rand.Intn(100) + 1)
		q = append(q, &LogEntry{
			ID: id,
		})
	}

	sort.Sort(q)

}

func TestRSM_Search(t *testing.T) {
	var r RSM
	var res []uint32
	for i := 0; i < 10; i++ {
		res = append(res, r.Search())
	}

	for i := 1; i < len(res); i++ {
		if res[i]-res[i-1] > 1 {
			t.Fatalf("not continuous")
		}
	}
}

func TestRSM_Commit(t *testing.T) {
	var r RSM

	num := make(map[uint32]*LogEntry)
	max := uint32(0)
	for i := 0; i < 100; i++ {
		logID := uint32(rand.Intn(1000))
		num[logID] = r.Insert(logID)
		if logID > max {
			max = logID
		}
	}

	pos := rand.Intn(len(num))
	i := 0
	for _, v := range num {
		i++
		if i == pos {
			break
		}

		r.Commit(v)
	}
}

func TestRSM_Insert(t *testing.T) {
	var r RSM

	num := make(map[uint32]*LogEntry)
	max := uint32(0)
	for i := 0; i < 100; i++ {
		logID := uint32(rand.Intn(1000))
		num[logID] = r.Insert(logID)
		if logID > max {
			max = logID
		}
	}

	for _, v := range num {
		if v.status != pending {
			t.Fatalf("status %d is not pending", v.status)
		}
	}

	count := 0
	for e := r.pending.Front(); e != nil; e = e.Next() {
		count++
	}

	if r.pending.Back().Value.(*LogEntry).ID != max {
		t.Fatalf("max incorrect")
	}

	if uint32(count) != max+1 {
		t.Fatalf("count incorrect %d != %d", count, max+1)
	}
}

func TestRSM_Free(t *testing.T) {
	var r RSM
	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func(i int) {
			r.Insert(uint32(i))
			wg.Done()
		}(i)
	}
	wg.Wait()

	newid := r.Search()

	if newid != 100 {
		t.Fatalf("search incorrect %d", newid)
	}

	fid := uint32(rand.Intn(100))
	e := r.Insert(fid)
	r.Free(e)

	newid = r.Search()
	if newid != fid {
		t.Fatalf("free incorrect")
	}
}
