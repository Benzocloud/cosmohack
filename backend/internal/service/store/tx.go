package store

// Tx — операции внутри WithLock; не берёт mutex повторно.
type Tx struct {
	s *Store
}

// WithLock выполняет fn под mutex store. Enqueue вызывать отсюда, если он мгновенный.
func (s *Store) WithLock(fn func(tx *Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fn(&Tx{s: s})
}

func (tx *Tx) Area(id string) (*Area, error) {
	return tx.s.getAreaLocked(id)
}

func (tx *Tx) PutArea(a Area) error {
	return tx.s.writeAreaLocked(a)
}

func (tx *Tx) Job(id string) (*Job, error) {
	return tx.s.getJobLocked(id)
}

func (tx *Tx) Jobs() ([]Job, error) {
	return tx.s.listJobsLocked()
}

func (tx *Tx) PutJobQueued(j Job) error {
	return tx.s.putJobQueuedLocked(j)
}

func (tx *Tx) DeleteJob(id string) error {
	return tx.s.deleteJobLocked(id)
}

func (tx *Tx) DeleteArea(id string) error {
	if _, err := tx.s.getAreaLocked(id); err != nil {
		return err
	}
	return tx.s.deleteAreaLocked(id)
}
