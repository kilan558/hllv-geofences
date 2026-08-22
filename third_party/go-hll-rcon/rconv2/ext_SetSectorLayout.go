package rconv2

import "context"

// SetSectorLayoutBySlice is the legacy HLL helper. HLL:V has a separate method
// that additionally requires a map ID, so this helper must not live on the
// shared embedded Connection type.
//
// Deprecated: Might be removed in a future release
func (c *HLLConnection) SetSectorLayoutBySlice(ctx context.Context, sectors []string) error {
	var values [5]string
	copy(values[:], sectors)
	return c.SetSectorLayout(ctx, values[0], values[1], values[2], values[3], values[4])
}
