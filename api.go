package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// responses

type Response struct {
	Data    any       `json:"data,omitempty"`
	Message string    `json:"message,omitempty"`
	Error   *AppError `json:"error,omitempty"`
}

func RespondOkWithCode(c *gin.Context, response Response, statusCode int) {
	r := gin.H{
		"ok": true,
	}
	if response.Data != nil {
		r["data"] = response.Data
	}
	if response.Message != "" {
		r["message"] = response.Message
	}
	c.JSON(statusCode, r)
}

func RespondOk(c *gin.Context, response Response) {
	RespondOkWithCode(c, response, http.StatusOK)
}

func RespondCreated(c *gin.Context, response Response) {
	RespondOkWithCode(c, response, http.StatusCreated)
}

func RespondError(c *gin.Context, response Response) {
	c.JSON(response.Error.Status, gin.H{
		"ok":    false,
		"error": response.Error.Error(),
	})
}

// errors

func ErrorByCurrentEnv(abstractMessage string, err error) error {
	ce := os.Getenv("CURRENT_ENV")

	switch ce {
	case "PROD":
		return errors.New(abstractMessage)
	case "DEV":
		return fmt.Errorf("%v: %w", abstractMessage, err)
	}

	return errors.New(abstractMessage)
}

type AppError struct {
	Status int    `json:"-"`
	Code   string `json:"code"`
	// абстрактный текст сообщения об ошибке; выводится всегда
	MessageAbstract string `json:"message"`
	// текст, дополняющий MessageAbstract; выводится при DEV=1
	ErrorDescribed error `json:"-"`
}

func (err *AppError) Error() string {
	ce := os.Getenv("CURRENT_ENV")

	switch ce {
	case "PROD":
		return err.MessageAbstract
	case "DEV":
		if err.ErrorDescribed == nil {
			return fmt.Sprintf("%v (no details)", err.MessageAbstract)
		} else {
			return fmt.Sprintf("%v: %v", err.MessageAbstract, err.ErrorDescribed)
		}
	}

	return err.MessageAbstract
}

func ErrBadRequest(messageAbstract string, err error) *AppError {
	if messageAbstract == "" {
		messageAbstract = "Bad request"
	}
	return &AppError{http.StatusBadRequest, "BAD_REQUEST", messageAbstract, err}
}

func ErrUnauthorized(messageAbstract string, err error) *AppError {
	if messageAbstract == "" {
		messageAbstract = "Unauthorized"
	}
	return &AppError{http.StatusUnauthorized, "UNAUTHORIZED", messageAbstract, err}
}

func ErrNotFound(messageAbstract string, err error) *AppError {
	if messageAbstract == "" {
		messageAbstract = "Resource not found"
	}
	return &AppError{http.StatusNotFound, "NOT_FOUND", messageAbstract, err}
}

func ErrUnprocessableEntity(messageAbstract string, err error) *AppError {
	if messageAbstract == "" {
		messageAbstract = "Unprocessable entity"
	}
	return &AppError{http.StatusUnprocessableEntity, "UNPROCESSABLE_ENTITY", messageAbstract, err}
}

func ErrInternal(messageAbstract string, err error) *AppError {
	if messageAbstract == "" {
		messageAbstract = "Internal server error"
	}
	return &AppError{http.StatusInternalServerError, "SERVER_ERROR", messageAbstract, err}
}
