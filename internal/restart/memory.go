package restart

// memory is a process's resident set, now and at its peak, as /proc reports
// them ("123456 kB").
type memory struct {
	rss, hwm string
}
