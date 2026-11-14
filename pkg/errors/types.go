package errors

import "net/http"

type RequiredError string

func (e RequiredError) Error() string   { return string(e) }
func (e RequiredError) StatusCode() int { return http.StatusBadRequest }

type ConflictError string

func (e ConflictError) Error() string   { return string(e) }
func (e ConflictError) StatusCode() int { return http.StatusConflict }

type NotFoundError string

func (e NotFoundError) Error() string   { return string(e) }
func (e NotFoundError) StatusCode() int { return http.StatusNotFound }

type InternalServerError string

func (e InternalServerError) Error() string   { return string(e) }
func (e InternalServerError) StatusCode() int { return http.StatusInternalServerError }

type BadRequestError string

func (e BadRequestError) Error() string   { return string(e) }
func (e BadRequestError) StatusCode() int { return http.StatusBadRequest }

type UnauthorizedError string

func (e UnauthorizedError) Error() string   { return string(e) }
func (e UnauthorizedError) StatusCode() int { return http.StatusUnauthorized }

type TooManyRequestsError string

func (e TooManyRequestsError) Error() string   { return string(e) }
func (e TooManyRequestsError) StatusCode() int { return http.StatusTooManyRequests }
