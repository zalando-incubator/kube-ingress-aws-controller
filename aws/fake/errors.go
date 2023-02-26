package fake

type fakeError struct{}

type notFoundError struct{}

type alreadyExistsError struct{}

func (e *alreadyExistsError) Error() string {
	return "This is a alreadyExistsError, for test purposes I hope."
}

func (e *notFoundError) Error() string {
	return "This is a notFoundError, for test purposes I hope."
}

func (e *fakeError) Error() string {
	return "This is a fake error, for test purposes."
}
