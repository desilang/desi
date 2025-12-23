package types

// Send and Sync are the core thread-safety traits in Desi.
//
// Send: A type is Send if it's safe to transfer ownership to another thread.
// Sync: A type is Sync if it's safe to share references across threads.
//
// These are automatically derived based on field types.

// IsSend returns true if the given type is safe to transfer ownership
// across thread boundaries (Send trait).
func IsSend(t T) bool {
	if t == nil {
		return true
	}

	// Primitives are always Send
	if isBasicType(t) {
		return true
	}

	switch v := t.(type) {
	// Collections are Send if their element types are Send
	case *List:
		return IsSend(v.Elem)
	case *Set:
		return IsSend(v.Elem)
	case *Dict:
		return IsSend(v.Key) && IsSend(v.Val)
	case *Tuple:
		for _, elem := range v.Elems {
			if !IsSend(elem) {
				return false
			}
		}
		return true

	// Structs are Send if all fields are Send
	case *Struct:
		for _, field := range v.Fields {
			if !IsSend(field.Type) {
				return false
			}
		}
		return true

	// Classes are Send if all fields are Send
	case *Class:
		for _, field := range v.Fields {
			if !IsSend(field.Type) {
				return false
			}
		}
		return true

	// Mutex is always Send (designed for cross-thread use)
	case *Mutex:
		return true

	// MutexGuard is NOT Send (can't transfer lock ownership across threads)
	case *MutexGuard:
		return false

	// Functions are Send
	case *Func:
		return true

	// Iterators are Send
	case *ListIter, *MapIter, *FilterIter:
		return true

	// Channels are Send if their element type is Send
	case *Channel:
		return IsSend(v.Elem)
	case *ChannelSender:
		return IsSend(v.Elem)
	case *ChannelReceiver:
		return IsSend(v.Elem)

	// Generic/unknown types - assume Send for now
	default:
		return true
	}
}

// IsSync returns true if the given type is safe to share references
// across thread boundaries (Sync trait).
func IsSync(t T) bool {
	if t == nil {
		return true
	}

	// Immutable primitives are always Sync
	if isBasicType(t) {
		return true
	}

	switch v := t.(type) {
	// Collections are Sync if their elements are Sync
	case *List:
		return IsSync(v.Elem)
	case *Set:
		return IsSync(v.Elem)
	case *Dict:
		return IsSync(v.Key) && IsSync(v.Val)
	case *Tuple:
		for _, elem := range v.Elems {
			if !IsSync(elem) {
				return false
			}
		}
		return true

	// Structs are Sync if all fields are Sync
	case *Struct:
		for _, field := range v.Fields {
			if !IsSync(field.Type) {
				return false
			}
		}
		return true

	// Classes are Sync if all fields are Sync
	case *Class:
		for _, field := range v.Fields {
			if !IsSync(field.Type) {
				return false
			}
		}
		return true

	// Mutex is Sync (designed for shared access)
	case *Mutex:
		return true

	// MutexGuard provides &mut access, so it's NOT Sync
	case *MutexGuard:
		return false

	// Functions are Sync
	case *Func:
		return true

	// Channels are Sync if their element type is Sync
	case *Channel:
		return IsSync(v.Elem)
	case *ChannelSender:
		return IsSync(v.Elem)
	case *ChannelReceiver:
		return IsSync(v.Elem)

	// Default: assume Sync
	default:
		return true
	}
}

// IsSendSync returns true if the type is both Send and Sync
func IsSendSync(t T) bool {
	return IsSend(t) && IsSync(t)
}

// isBasicType returns true for primitive/basic types like int, float, bool, str
func isBasicType(t T) bool {
	if t == nil {
		return false
	}
	// Check if it's one of the singleton basic types or a *basic type
	switch t {
	case Int, Float, Bool, Str, None, Any, Type, Char, Decimal, USize, ISize, File:
		return true
	}
	// Check for any *basic type (sized integers, etc)
	if _, ok := t.(*basic); ok {
		return true
	}
	return false
}
