package satehdns

func (h *SatehHandler) Ready() bool {
	h.Lock()
	defer h.Unlock()
	return h.state == "ready"
}
