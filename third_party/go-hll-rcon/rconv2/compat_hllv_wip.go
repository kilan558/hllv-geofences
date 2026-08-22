package rconv2

// MapName is referenced by shared generated commands in HLL:V PR #6, but the
// generator currently only emits the game-specific HLLMapName/HLLVMapName
// types. A string-backed compatibility type preserves the RCON wire format.
type MapName string
