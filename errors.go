package main

import "errors"

var ErrCannotDeleteRootFolder = errors.New("Cannot delete root folder")

var ErrNotSupportedExtension = errors.New("Not supported extension")
var ErrNotSupportedSize = errors.New("Not supported size")

var ErrInvalidInt = errors.New("Invalid integer value")
var ErrLessThanMin = errors.New("Value is less than required")
var ErrMoreThanMax = errors.New("Value is more than required")

var ErrS3IsNotAvailable = errors.New("S3 is not available")
