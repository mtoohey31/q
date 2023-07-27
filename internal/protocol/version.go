package protocol

// Version is the current version of the protocol.
//
// Minor version increments will be made for backwards-compatible changes,
// such as additions of extra fields or new messages that are not required to
// be handled.
//
// Major version increments will be made for backwards-incompatible changes,
// such as changes to the types of existing messages.
var Version = "0.5.0"
