package yum

// Converts any struct to a pointer to that struct
func Ptr[T any](item T) *T {
	return &item
}
