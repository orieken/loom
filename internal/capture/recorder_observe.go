package capture

// observationEvents re-hashes the open hunks that this call can see and
// returns an event for each hunk whose hash changed since the tail last saw
// it. An unchanged hunk writes nothing (change-only emission).
func (c *hookCall) observationEvents(tail []SessionEvent, main SessionEvent) []SessionEvent {
	if len(tail) == 0 {
		return nil
	}
	ledger := ledgerFromTail(tail, c.now, c.cfg.Window)
	var events []SessionEvent
	for _, group := range groupByPath(c.hunksToRehash(ledger, main)) {
		if c.files.isSpent() {
			break
		}
		events = append(events, c.observeGroup(group, main)...)
	}
	return events
}

// hunksToRehash applies the scope rules: a prompt sees every path, a file
// call sees its own path, any other call sees none. A file writer result
// skips its own path, because the agent write explains any change there.
func (c *hookCall) hunksToRehash(ledger *HunkLedger, main SessionEvent) []openHunk {
	switch {
	case c.obs.Kind == EventPrompt:
		return ledger.newest("", MaxRehashedHunks)
	case main.PathRef == "" || c.isFileWriterResult():
		return nil
	}
	return ledger.newest(main.PathRef, MaxRehashedHunks)
}

// observeGroup reads one file and hashes every selected range from the same
// split lines.
func (c *hookCall) observeGroup(group pathGroup, main SessionEvent) []SessionEvent {
	read := c.files.read(c.ctx, group.path)
	if read.err != nil {
		return nil
	}
	var events []SessionEvent
	for _, hunk := range group.hunks {
		hash := c.currentHash(read.splitLines(), hunk.facts)
		if hash == hunk.facts.PostHash {
			continue
		}
		if event, err := c.observed(hunk, hash, main); err == nil {
			events = append(events, event)
		}
	}
	return events
}

// currentHash hashes the range now. A range beyond the end of the file means
// the lines are gone, which hashes as an empty text.
func (c *hookCall) currentHash(lines [][]byte, facts HunkFacts) string {
	hash, isInside := c.hasher.HashSplitLines(lines, facts.RangeStart, facts.RangeEnd)
	if !isInside {
		return c.hasher.Hash(HashLines, "")
	}
	return hash
}

func (c *hookCall) observed(hunk openHunk, hash string, main SessionEvent) (SessionEvent, error) {
	id, err := c.deps.NewEventID()
	if err != nil {
		return SessionEvent{}, err
	}
	return NewSessionEvent(id, c.now, EventHunkObserved).
		WithTool(ObserverTool).
		WithSessionRef(main.SessionRef).
		WithPath(hunk.pathRef, hunk.path).
		WithHunk(HunkFacts{RangeStart: hunk.facts.RangeStart, RangeEnd: hunk.facts.RangeEnd, PostHash: hash}), nil
}
