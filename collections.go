package main

// nonNilCopy returns a copy of list that's never nil, even when list is —
// a nil slice marshals to JSON null, and null.map() throws in the frontend.
func nonNilCopy[T any](list []T) []T {
	out := make([]T, len(list))
	copy(out, list)
	return out
}

// upsert replaces the first element of list matching key with item, or
// appends item if none match.
func upsert[T any](list []T, key func(T) bool, item T) []T {
	for i := range list {
		if key(list[i]) {
			list[i] = item
			return list
		}
	}
	return append(list, item)
}

// removeIf returns list with every element matching drop removed.
func removeIf[T any](list []T, drop func(T) bool) []T {
	filtered := list[:0]
	for _, item := range list {
		if !drop(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// find returns the first element of list matching, or the zero value and
// false if none do.
func find[T any](list []T, match func(T) bool) (T, bool) {
	for _, item := range list {
		if match(item) {
			return item, true
		}
	}
	var zero T
	return zero, false
}
