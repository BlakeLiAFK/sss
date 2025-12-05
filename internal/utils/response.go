package utils

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
)

// S3Error S3错误响应
type S3Error struct {
	XMLName   xml.Name `xml:"Error"`
	Code      string   `xml:"Code"`
	Message   string   `xml:"Message"`
	Resource  string   `xml:"Resource"`
	RequestID string   `xml:"RequestId"`
}

// 预定义错误
var (
	ErrNoSuchBucket         = S3Error{Code: "NoSuchBucket", Message: "The specified bucket does not exist"}
	ErrNoSuchKey            = S3Error{Code: "NoSuchKey", Message: "The specified key does not exist"}
	ErrBucketAlreadyExists  = S3Error{Code: "BucketAlreadyExists", Message: "The requested bucket name is not available"}
	ErrBucketNotEmpty       = S3Error{Code: "BucketNotEmpty", Message: "The bucket you tried to delete is not empty"}
	ErrAccessDenied         = S3Error{Code: "AccessDenied", Message: "Access Denied"}
	ErrSignatureDoesNotMatch = S3Error{Code: "SignatureDoesNotMatch", Message: "The request signature we calculated does not match the signature you provided"}
	ErrInvalidAccessKeyId   = S3Error{Code: "InvalidAccessKeyId", Message: "The AWS Access Key Id you provided does not exist"}
	ErrNoSuchUpload         = S3Error{Code: "NoSuchUpload", Message: "The specified upload does not exist"}
	ErrInvalidPart          = S3Error{Code: "InvalidPart", Message: "One or more of the specified parts could not be found"}
	ErrInvalidArgument      = S3Error{Code: "InvalidArgument", Message: "Invalid Argument"}
	ErrInternalError        = S3Error{Code: "InternalError", Message: "We encountered an internal error. Please try again."}
	ErrMethodNotAllowed     = S3Error{Code: "MethodNotAllowed", Message: "The specified method is not allowed against this resource"}
	ErrMalformedJSON        = S3Error{Code: "MalformedJSON", Message: "The JSON provided was not well-formed"}
	ErrEntityTooLarge      = S3Error{Code: "EntityTooLarge", Message: "Your proposed upload exceeds the maximum allowed size"}
	ErrBadDigest           = S3Error{Code: "BadDigest", Message: "The Content-MD5 you specified did not match what we received"}
)

// WriteError 写入错误响应
func WriteError(w http.ResponseWriter, err S3Error, statusCode int, resource string) {
	err.Resource = resource
	err.RequestID = GenerateRequestID()

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(statusCode)
	xml.NewEncoder(w).Encode(err)
}

// WriteXML 写入XML响应
func WriteXML(w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(statusCode)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(v)
}

// GenerateRequestID 生成请求ID
func GenerateRequestID() string {
	return GenerateID(16)
}

// ParseJSONBody 解析JSON请求体
func ParseJSONBody(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// WriteJSONResponse 写入JSON响应
func WriteJSONResponse(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// WriteErrorResponse 写入JSON错误响应
func WriteErrorResponse(w http.ResponseWriter, code, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	})
}
