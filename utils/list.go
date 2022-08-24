package utils

// ListItem with key and value
type ListItem struct {
	K string
	V string
}

// List slice
type List []ListItem

// Get item's value
func (l List) Get(key string) (string, bool) {
	for _, item := range l {
		if item.K == key {
			return item.V, true
		}
	}
	return "", false
}

// ForEach item
func (l List) ForEach(handler func(key, value string)) {
	for _, item := range l {
		handler(item.K, item.V)
	}
}
