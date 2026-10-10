package encstore

// Prefix equivalence is platform-supplied. Only the root/family prefix is
// normalized: canonical object IDs and suffixes must still parse exactly.
func foldedFamilyEntry(name, base string, prefix func(string, string) (string, bool, error)) (bool, error) {
	for _, whole := range []string{base, base + ".gse-lock"} {
		tail, match, err := prefix(name, whole)
		if err != nil {
			return false, err
		}
		if match && tail == "" {
			return true, nil
		}
	}
	head := base + ".gse-"
	tail, match, err := prefix(name, head)
	if err != nil {
		return false, err
	}
	if !match {
		return false, nil
	}
	return familyEntry(head+tail, base)
}

// NTFS has a 255 UTF-16-unit component limit. The longest generated suffix is
// ".gse-" + 64 hexadecimal units + ".retained" = 78 units. Bound every user
// target name conservatively before it can create a stable lock or ask for a
// credential; even a first short .base publication must reserve future names.
func windowsFamilyNameBound(name string) error {
	units := 0
	for _, r := range name {
		units++
		if r > 0xffff {
			units++
		}
		if units > 177 {
			return ErrUnsupported
		}
	}
	return nil
}
