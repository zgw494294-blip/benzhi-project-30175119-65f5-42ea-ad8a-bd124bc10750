package repository

import "errors"

var ErrNotFound = errors.New("案件不存在")
var ErrDuplicate = errors.New("记录已存在")
