package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type ResponseErrorString string
type ResponseErrorMsg string
type ResponseError struct {
	ErrorCode ResponseErrorString
	ErrorMsg  ResponseErrorMsg
	HttpError int
}

type responseErrorType map[ResponseErrorString]ResponseError

var responseErrors responseErrorType

func init() {
	responseErrors = make(responseErrorType)
}

func addResponseError(errCode ResponseErrorString, errMsg ResponseErrorMsg, httpCode int) {
	responseErrors[errCode] = ResponseError{errCode, errMsg, httpCode}
}

const (
	MISSING_REQUIRED_DATA ResponseErrorString = "MISSING_REQUIRED_DATA"
	RPC_SERVER_ERROR      ResponseErrorString = "RPC_SERVER_ERROR"
	SAVE_SESSION_ERROR    ResponseErrorString = "SAVE_SESSION_ERROR"
	JSON_ENCODE_ERROR     ResponseErrorString = "JSON_ENCODE_ERROR"
)

func init() {
	addResponseError("MISSING_REQUIRED_DATA", "Some data that is required was missing", http.StatusBadRequest)
	addResponseError("RPC_SERVER_ERROR", "Could not reach RPC server", http.StatusInternalServerError)
	addResponseError("SAVE_SESSION_ERROR", "Could not save session", http.StatusInternalServerError)
	addResponseError("JSON_ENCODE_ERROR", "Could not encode json reply", http.StatusInternalServerError)
}

func responseError(w http.ResponseWriter, errCode ResponseErrorString) {
	errResponse, ok := responseErrors[errCode]
	if !ok {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	responseData := make(map[string]string)
	//responseData["Token"] = ""
	responseData["Status"] = string(errResponse.ErrorCode)
	responseData["Message"] = string(errResponse.ErrorMsg)

	responseJson, err := json.Marshal(responseData)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(errResponse.HttpError)
	fmt.Fprintln(w, string(responseJson))
}

func printErrorsForDoc() {
	errArray := make([]ResponseError, 0)
	for _, v := range responseErrors {
		errArray = append(errArray, v)
	}
	jsonStr, err := json.Marshal(errArray)
	if err != nil {
		fmt.Printf("Could not marshall json: %v\n", err)
		return
	}
	fmt.Println(string(jsonStr))

}
