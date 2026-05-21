package core

import "fmt"

func applyDuplicateFieldPolicy(ev *Event, policy DuplicateFieldPolicy) error {
	if ev == nil {
		return nil
	}

	if policy == DropDuplicateAttr {
		policy = CanonicalWins
	}

	ev.MuLock()
	defer ev.MuUnlock()

	if len(ev.Attrs) == 0 {
		return nil
	}

	out := make([]Attr, 0, len(ev.Attrs))
	for _, a := range ev.Attrs {
		if !isCanonicalKey(a.Key) {
			out = append(out, a)
			continue
		}

		switch policy {
		case CanonicalWins:
			continue
		case AttrWins:
			if ev.applyCanonical(a) {
				continue
			}
			a.Key = "attrs." + a.Key
			out = append(out, a)
		case KeepBothUnderAttrs:
			a.Key = "attrs." + a.Key
			out = append(out, a)
		case ErrorOnDuplicate:
			return &DuplicateFieldError{Key: a.Key}
		default:
			return fmt.Errorf("loxa: unknown duplicate field policy %d", policy)
		}
	}

	ev.Attrs = out
	return nil
}
