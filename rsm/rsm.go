package rsm

import (
	"concordia/util"
	"container/list"
	"sort"
	"sync"
)

const (
	pending = iota
	free
	committed
	executing
	executed
)

var Logger = util.NewLogger("<rsm>")

type LogEntry struct {
	ID      uint32
	Value   string
	element *list.Element
	status  uint8
}

type CommittedQueue []*LogEntry

func (q CommittedQueue) Len() int {
	return len(q)
}

func (q CommittedQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

func (q CommittedQueue) Less(i, j int) bool {
	return q[i].ID < q[j].ID
}

type RSM struct {
	// committed queue
	Committed CommittedQueue
	// lock to protect whole rsm obj
	lock sync.Mutex
	// pending queue
	pending list.List
}

// given a log entry id, search it in pending queue,
// if not found, create it and all the entries before it if possible
// if log id is obsolete, return nil
func (r *RSM) Insert(logID uint32) *LogEntry {
	r.lock.Lock()
	defer r.lock.Unlock()

	var entry *LogEntry

	// check if is already in committed queue
	if r.Committed.Len() != 0 && r.Committed[r.Committed.Len()-1].ID >= logID {
		return nil
	}

	// if pending queue is empty, create the given entry and entries before it
	if r.pending.Len() == 0 {
		var id uint32
		if r.Committed.Len() != 0 {
			id = r.Committed[r.Committed.Len()-1].ID + 1
		}

		for i := id; i <= logID; i++ {
			entry = &LogEntry{
				ID:     i,
				status: free,
			}
			entry.element = r.pending.PushBack(entry)
		}

		entry.status = pending

		Logger.Debugf("rsm %p has empty pending queue, add new entries for log %d - %d, "+
			"returning entry %p with id %d", r, 0, logID, entry, entry.ID)
		return entry
	}

	// search from the front of pending queue
	e := r.pending.Front()
	for ; e != nil; e = e.Next() {
		if e.Value.(*LogEntry).ID >= logID {
			break
		}
	}

	// if requesting entry is ahead of pending queue's back, add those entries
	if e == nil {
		for i := r.pending.Back().Value.(*LogEntry).ID + 1; i <= logID; i++ {
			entry = &LogEntry{
				ID:     i,
				status: free,
			}
			entry.element = r.pending.PushBack(entry)
		}

		entry.status = pending

		Logger.Debugf("rsm %p insert a new entry %p with id %d", r, entry, entry.ID)
		return entry
	}

	// if a correspoding entry is found, check its status
	entry = e.Value.(*LogEntry)
	if entry.ID == logID {
		if entry.status == committed {
			// this entry has been committed before
			return nil
		}
		// if this entry is being pending, consensus algorithm outside should make sure
		// there will be only one commit for this entry
		Logger.Debugf("rsm %p has found exact log entry %p with id %d status %d",
			r, entry, entry.ID, entry.status)
		entry.status = pending
		return entry
	} else {
		// fill in the gap
		pentry := e.Prev().Value.(*LogEntry)
		for id := pentry.ID; id <= logID; id++ {
			entry = &LogEntry{
				ID:     id,
				status: free,
			}
			entry.element = r.pending.InsertAfter(entry, pentry.element)
			pentry = entry
		}

		entry.status = pending

		Logger.Debugf("rsm %p has created log entry %d with status %d",
			r, entry.ID, entry.status)
		return entry
	}
}

// set the given entry to committed status
// and move it to committed queue if possible
func (r *RSM) Commit(e *LogEntry) {
	Logger.Debugf("rsm %p is about to commit entry %p with log id %d and value %s",
		r, e, e.ID, e.Value)
	r.lock.Lock()
	defer r.lock.Unlock()

	if e.status == committed {
		// should not have committed more than once
		Logger.Panicf("double commit occurred, log %d value %s", e.ID, e.Value)
		return
	}

	e.status = committed

	// check if there is any entry could be committed
	var next *list.Element
	var newin bool
	for v := r.pending.Front(); v != nil; v = next {
		if v.Value.(*LogEntry).status != committed {
			break
		}

		next = v.Next()
		re := r.pending.Remove(v)
		r.Committed = append(r.Committed, re.(*LogEntry))

		Logger.Debugf("rsm %p has pushed log entry %p with id %d and value %s into committed queue",
			r, e, e.ID, e.Value)
	}

	// sort
	if newin {
		sort.Sort(r.Committed)
	}
}

func (r *RSM) Free(e *LogEntry) {
	r.lock.Lock()
	if e.status != committed {
		e.status = free
	}
	r.lock.Unlock()
}

// try to find a log entry id which is free
// but doesn't get its ownership
func (r *RSM) Search() uint32 {
	r.lock.Lock()
	defer r.lock.Unlock()

	var id uint32
	if r.pending.Len() == 0 {
		if r.Committed.Len() != 0 {
			id = r.Committed[r.Committed.Len()-1].ID + 1
		}

		r.pending.PushFront(&LogEntry{
			ID:     id,
			status: free,
		})

		return id
	}

	for e := r.pending.Front(); e != nil; e = e.Next() {
		if e.Value.(*LogEntry).status == free {
			return e.Value.(*LogEntry).ID
		}
	}

	// not found, return last one + 1
	id = r.pending.Back().Value.(*LogEntry).ID
	r.pending.PushBack(&LogEntry{
		ID:     id + 1,
		status: free,
	})

	return id + 1
}

func (r *RSM) NextCommitted() (uint32, string) {
	r.lock.Lock()
	defer r.lock.Unlock()

	for i := 0; i < r.Committed.Len(); i++ {
		if r.Committed[i].status == committed {
			r.Committed[i].status = executing
			return r.Committed[i].ID, r.Committed[i].Value
		}
	}

	return 0, ""
}

func (r *RSM) ExecutionSucceed(logID uint32) {
	r.lock.Lock()
	defer r.lock.Unlock()

	for i := 0; i < r.Committed.Len(); i++ {
		if r.Committed[i].ID == logID {
			r.Committed[i].status = executed
		}
	}
}

func (r *RSM) ExecutionFail(logID uint32) {
	r.lock.Lock()
	defer r.lock.Unlock()

	for i := 0; i < r.Committed.Len(); i++ {
		if r.Committed[i].ID == logID {
			r.Committed[i].status = committed
		}
	}
}

func (r *RSM) Recover(logID uint32, value string) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.Committed.Len() != 0 {
		Logger.Panicf("rsm %p has non-empty (%d) committed queue when recovering",
			r, r.Committed.Len())
		return
	}

	r.Committed = append(r.Committed, &LogEntry{
		ID:     logID,
		Value:  value,
		status: committed,
	})

	sort.Sort(r.Committed)
}
