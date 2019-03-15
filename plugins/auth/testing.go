package auth

func compareErrs(expectedErr string, actual error) bool {
	if actual == nil {
		if expectedErr == "" {
			return true
		}
		return false
	}

	return expectedErr == actual.Error()
}
