package common

type HeapSlice []TaskStrsByHostname

func (h HeapSlice) Len() int {
	return len(h)
}
func (h HeapSlice) Less(i int, j int) bool {
	return h[i].Schedule < h[j].Schedule
}
func (h HeapSlice) Swap(i int, j int) {
	h[i], h[j] = h[j], h[i]
}
func (h *HeapSlice) Push(x interface{}) {
	taskByHostname := x.(TaskStrsByHostname)
	*h = append(*h, taskByHostname)
}
func (h *HeapSlice) Pop() interface{} {
	old := *h
	n := len(old)
	taskByHostname := old[n-1]
	*h = old[0 : n-1]
	return taskByHostname
}
