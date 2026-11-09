package domain

import "fmt"

type Error struct {
	Code, Message string
	Status        int
	Details       any
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }
func Fail(code string, status int, message string) error {
	return &Error{Code: code, Status: status, Message: message}
}
func Validation(field string) error {
	return Fail("VALIDATION", 422, fmt.Sprintf("Некорректное поле: %s.", field))
}

var NotFound = Fail("NOT_FOUND", 404, "Объект не найден.")
var Forbidden = Fail("FORBIDDEN", 403, "Доступ запрещён.")
var AuthRequired = Fail("AUTH_REQUIRED", 401, "Требуется вход.")
