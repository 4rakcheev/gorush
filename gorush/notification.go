package gorush

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/google/go-gcm"
	apns "github.com/sideshow/apns2"
	"github.com/sideshow/apns2/certificate"
	"github.com/sideshow/apns2/payload"
)

// D provide string array
type D map[string]interface{}

const (
	// ApnsPriorityLow will tell APNs to send the push message at a time that takes
	// into account power considerations for the device. Notifications with this
	// priority might be grouped and delivered in bursts. They are throttled, and
	// in some cases are not delivered.
	ApnsPriorityLow = 5

	// ApnsPriorityHigh will tell APNs to send the push message immediately.
	// Notifications with this priority must trigger an alert, sound, or badge on
	// the target device. It is an error to use this priority for a push
	// notification that contains only the content-available key.
	ApnsPriorityHigh = 10
)

// Alert is APNs payload
type Alert struct {
	Action       string   `json:"action,omitempty"`
	ActionLocKey string   `json:"action-loc-key,omitempty"`
	Body         string   `json:"body,omitempty"`
	LaunchImage  string   `json:"launch-image,omitempty"`
	LocArgs      []string `json:"loc-args,omitempty"`
	LocKey       string   `json:"loc-key,omitempty"`
	Title        string   `json:"title,omitempty"`
	Subtitle     string   `json:"subtitle,omitempty"`
	TitleLocArgs []string `json:"title-loc-args,omitempty"`
	TitleLocKey  string   `json:"title-loc-key,omitempty"`
}

// RequestPush support multiple notification request.
type RequestPush struct {
	Notifications []PushNotification `json:"notifications" binding:"required"`
}

// PushNotification is single notification request
type PushNotification struct {
	// Common
	Tokens           []string `json:"tokens" binding:"required"`
	Platform         int      `json:"platform" binding:"required"`
	Message          string   `json:"message,omitempty"`
	Title            string   `json:"title,omitempty"`
	Priority         string   `json:"priority,omitempty"`
	ContentAvailable bool     `json:"content_available,omitempty"`
	Sound            string   `json:"sound,omitempty"`
	Data             D        `json:"data,omitempty"`
	Retry            int      `json:"retry,omitempty"`

	// Android
	APIKey                string           `json:"api_key,omitempty"`
	To                    string           `json:"to,omitempty"`
	CollapseKey           string           `json:"collapse_key,omitempty"`
	DelayWhileIdle        bool             `json:"delay_while_idle,omitempty"`
	TimeToLive            *uint            `json:"time_to_live,omitempty"`
	RestrictedPackageName string           `json:"restricted_package_name,omitempty"`
	DryRun                bool             `json:"dry_run,omitempty"`
	Notification          gcm.Notification `json:"notification,omitempty"`

	// iOS
	Expiration int64    `json:"expiration,omitempty"`
	ApnsID     string   `json:"apns_id,omitempty"`
	Topic      string   `json:"topic,omitempty"`
	Badge      *int     `json:"badge,omitempty"`
	Category   string   `json:"category,omitempty"`
	URLArgs    []string `json:"url-args,omitempty"`
	Alert      Alert    `json:"alert,omitempty"`
}


// The possible Reason error codes returned from Google.
// From google table https://developers.google.com/cloud-messaging/http-server-ref#error-codes and Remote Notification Programming Guide.
const (
	//Check that the request contains a registration ID (either in the registration_id parameter in a plain text message,
	// or in the registration_ids field in JSON).
	ReasonMissingRegistration = "MissingRegistration"

	// Check the formatting of the registration ID that you pass to the server.
	// Make sure it matches the registration ID the phone receives in the com.google.android.c2dm.intent.
	// REGISTRATION intent and that you're not truncating it or adding additional characters
	ReasonInvalidRegistration = "InvalidRegistration"

	//A registration ID is tied to a certain group of senders.
	//When an application registers for GCM usage, it must specify which senders are allowed to send messages.
	// Make sure you're using one of those when trying to send messages to the device.
	// If you switch to a different sender, the existing registration IDs won't work.
	ReasonMismatchSenderId = "MismatchSenderId"

	//An existing registration ID may cease to be valid in a number of scenarios, including:
	//If the application manually unregisters by issuing a com.google.android.c2dm.intent.UNREGISTER intent.
	//If the application is automatically unregistered, which can happen (but is not guaranteed) if the user uninstalls the application.
	//If the registration ID expires. Google might decide to refresh registration IDs.
	//If the application is updated but the new version does not have a broadcast receiver configured to receive com.google.android.c2dm.intent.RECEIVE intents.
	//For all these cases, you should remove this registration ID from the 3rd-party server and stop using it to send messages
	ReasonNotRegistered = "NotRegistered"

	//The total size of the payload data that is included in a message can't exceed 4096 bytes.
	//Note that this includes both the size of the keys as well as the values
	ReasonMessageTooBig = "MessageTooBig"

	//The payload data contains a key (such as from or any value prefixed by google.)
	//that is used internally by GCM in the com.google.android.c2dm.intent.RECEIVE Intent and cannot be used.
	//Note that some words (such as collapse_key) are also used by GCM but are allowed in the payload,
	// in which case the payload value will be overridden by the GCM value
	ReasonInvalidDataKey = "InvalidDataKey"

	//The value for the Time to Live field must be an integer representing a duration in seconds between 0 and 2,419,200 (4 weeks)
	ReasonInvalidTtl = "InvalidTtl"

	//The server encountered an error while trying to process the request.
	// You could retry the same request (obeying the requirements listed in the Timeout section),
	// but if the error persists, please report the problem in the android-gcm group.
	// Happens when the HTTP status code is 500, or when the error field of a JSON object in the results array is
	ReasonInternalServerError = "InternalServerError"

	// A message was addressed to a registration ID whose package name did not match the value passed in the request.
	ReasonInvalidPackageName = "InvalidPackageName"

	//The server couldn't process the request in time. Retry the same reques
	ReasonUnavailable = "Unavailable"

	//The rate of messages to a particular device is too high.
	//Reduce the number of messages sent to this device and do not immediately retry sending to this device.
	ReasonDeviceMessageRateExceeded = "DeviceMessageRateExceeded"

	//The rate of messages to subscribers to a particular topic is too high.
	//Reduce the number of messages sent for this topic, and do not immediately retry sending.
	ReasonTopicsMessageRateExceeded = "TopicsMessageRateExceeded"

	//A message targeted to an iOS device could not be sent because the required APNs SSL certificate was not
	//uploaded or has expired. Check the validity of your development and production certificates.
	ReasonInvalidApnsCredential = "InvalidApnsCredential"
)

// Response represents a result from the APNs gateway indicating whether a
// notification was accepted or rejected and (if applicable) the metadata
// surrounding the rejection.
type GoogleResponse struct {

	// The Google error string indicating the reason for the notification failure (if
	// any). The error code is specified as a string. For a list of possible
	// values, see the Reason constants above.
	Reason string
}


// CheckMessage for check request message
func CheckMessage(req PushNotification) error {
	var msg string

	if len(req.Tokens) == 0 {
		msg = "the message must specify at least one registration ID"
		LogAccess.Debug(msg)
		return errors.New(msg)
	}

	if len(req.Tokens) == PlatFormIos && len(req.Tokens[0]) == 0 {
		msg = "the token must not be empty"
		LogAccess.Debug(msg)
		return errors.New(msg)
	}

	if req.Platform == PlatFormAndroid && len(req.Tokens) > 1000 {
		msg = "the message may specify at most 1000 registration IDs"
		LogAccess.Debug(msg)
		return errors.New(msg)
	}

	// ref: https://developers.google.com/cloud-messaging/http-server-ref
	if req.Platform == PlatFormAndroid && req.TimeToLive != nil && (*req.TimeToLive < uint(0) || uint(2419200) < *req.TimeToLive) {
		msg = "the message's TimeToLive field must be an integer " +
			"between 0 and 2419200 (4 weeks)"
		LogAccess.Debug(msg)
		return errors.New(msg)
	}

	return nil
}

// SetProxy only working for GCM server.
func SetProxy(proxy string) error {

	proxyURL, err := url.ParseRequestURI(proxy)

	if err != nil {
		return err
	}

	http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	LogAccess.Debug("Set http proxy as " + proxy)

	return nil
}

// CheckPushConf provide check your yml config.
func CheckPushConf() error {
	if !PushConf.Ios.Enabled && !PushConf.Android.Enabled {
		return errors.New("Please enable iOS or Android config in yml config")
	}

	if PushConf.Ios.Enabled {
		if PushConf.Ios.KeyPath == "" {
			return errors.New("Missing iOS certificate path")
		}
	}

	if PushConf.Android.Enabled {
		if PushConf.Android.APIKey == "" {
			return errors.New("Missing Android API Key")
		}
	}

	return nil
}

// InitAPNSClient use for initialize APNs Client.
func InitAPNSClient() error {
	if PushConf.Ios.Enabled {
		var err error
		ext := filepath.Ext(PushConf.Ios.KeyPath)

		switch ext {
		case ".p12":
			CertificatePemIos, err = certificate.FromP12File(PushConf.Ios.KeyPath, PushConf.Ios.Password)
		case ".pem":
			CertificatePemIos, err = certificate.FromPemFile(PushConf.Ios.KeyPath, PushConf.Ios.Password)
		default:
			err = errors.New("wrong certificate key extension")
		}

		if err != nil {
			LogError.Error("Cert Error:", err.Error())

			return err
		}

		if PushConf.Ios.Production {
			ApnsClient = apns.NewClient(CertificatePemIos).Production()
		} else {
			ApnsClient = apns.NewClient(CertificatePemIos).Development()
		}
	}

	return nil
}

// InitWorkers for initialize all workers.
func InitWorkers(workerNum int64, queueNum int64) {
	LogAccess.Debug("worker number is ", workerNum, ", queue number is ", queueNum)
	QueueNotification = make(chan PushNotification, queueNum)
	for i := int64(0); i < workerNum; i++ {
		go startWorker()
	}
}

func startWorker() {
	for {
		notification := <-QueueNotification
		switch notification.Platform {
		case PlatFormIos:
			PushToIOS(notification)
		case PlatFormAndroid:
			PushToAndroid(notification)
		}
	}
}

// queueNotification add notification to queue list.
func queueNotification(req RequestPush) int {
	var count int
	for _, notification := range req.Notifications {
		switch notification.Platform {
		case PlatFormIos:
			if !PushConf.Ios.Enabled {
				continue
			}
		case PlatFormAndroid:
			if !PushConf.Android.Enabled {
				continue
			}
		}
		QueueNotification <- notification

		count += len(notification.Tokens)
	}

	StatStorage.AddTotalCount(int64(count))

	return count
}

func iosAlertDictionary(payload *payload.Payload, req PushNotification) *payload.Payload {
	// Alert dictionary

	if len(req.Title) > 0 {
		payload.AlertTitle(req.Title)
	}

	if len(req.Alert.Title) > 0 {
		payload.AlertTitle(req.Alert.Title)
	}

	// Apple Watch & Safari display this string as part of the notification interface.
	if len(req.Alert.Subtitle) > 0 {
		payload.AlertSubtitle(req.Alert.Subtitle)
	}

	if len(req.Alert.TitleLocKey) > 0 {
		payload.AlertTitleLocKey(req.Alert.TitleLocKey)
	}

	if len(req.Alert.LocArgs) > 0 {
		payload.AlertLocArgs(req.Alert.LocArgs)
	}

	if len(req.Alert.TitleLocArgs) > 0 {
		payload.AlertTitleLocArgs(req.Alert.TitleLocArgs)
	}

	if len(req.Alert.Body) > 0 {
		payload.AlertBody(req.Alert.Body)
	}

	if len(req.Alert.LaunchImage) > 0 {
		payload.AlertLaunchImage(req.Alert.LaunchImage)
	}

	if len(req.Alert.LocKey) > 0 {
		payload.AlertLocKey(req.Alert.LocKey)
	}

	if len(req.Alert.Action) > 0 {
		payload.AlertAction(req.Alert.Action)
	}

	if len(req.Alert.ActionLocKey) > 0 {
		payload.AlertActionLocKey(req.Alert.ActionLocKey)
	}

	// General

	if len(req.Category) > 0 {
		payload.Category(req.Category)
	}

	return payload
}

// GetIOSNotification use for define iOS notificaiton.
// The iOS Notification Payload
// ref: https://developer.apple.com/library/content/documentation/NetworkingInternet/Conceptual/RemoteNotificationsPG/PayloadKeyReference.html#//apple_ref/doc/uid/TP40008194-CH17-SW1
func GetIOSNotification(req PushNotification) *apns.Notification {
	notification := &apns.Notification{
		ApnsID: req.ApnsID,
		Topic:  req.Topic,
	}

	if req.Expiration > 0 {
		notification.Expiration = time.Unix(req.Expiration, 0)
	}

	if len(req.Priority) > 0 && req.Priority == "normal" {
		notification.Priority = apns.PriorityLow
	}

	payload := payload.NewPayload()

	// add alert object if message length > 0
	if len(req.Message) > 0 {
		payload.Alert(req.Message)
	}

	// zero value for clear the badge on the app icon.
	if req.Badge != nil && *req.Badge >= 0 {
		payload.Badge(*req.Badge)
	}

	if len(req.Sound) > 0 {
		payload.Sound(req.Sound)
	}

	if req.ContentAvailable {
		payload.ContentAvailable()
	}

	if len(req.URLArgs) > 0 {
		payload.URLArgs(req.URLArgs)
	}

	for k, v := range req.Data {
		payload.Custom(k, v)
	}

	payload = iosAlertDictionary(payload, req)

	notification.Payload = payload

	return notification
}

// PushToIOS provide send notification to APNs server.
func PushToIOS(req PushNotification) bool {
	var isError bool
	_, isError = PushToIOSWithErrorResult(req)
	return isError
}

// PushToIOSWithErrorResult provide send notification to APNs server and return response array for failed requests.
func PushToIOSWithErrorResult(req PushNotification)  (*map[string]*apns.Response,bool) {
	LogAccess.Debug("Start push notification for iOS")

	var retryCount = 0
	var maxRetry = PushConf.Ios.MaxRetry

	if req.Retry > 0 && req.Retry < maxRetry {
		maxRetry = req.Retry
	}

Retry:
	var isError = false
	var newTokens []string
	var returnResultList map[string]*apns.Response
	returnResultList = make(map[string]*apns.Response)

	notification := GetIOSNotification(req)

	for _, token := range req.Tokens {
		notification.DeviceToken = token

		// send ios notification
		res, err := ApnsClient.Push(notification)

		if err != nil {
			// apns server error
			LogPush(FailedPush, token, req, err)
			StatStorage.AddIosError(1)
			newTokens = append(newTokens, token)
			returnResultList[token] = res
			isError = true
			continue
		}

		if res.StatusCode != 200 {
			// error message:
			// ref: https://github.com/sideshow/apns2/blob/master/response.go#L14-L65
			LogPush(FailedPush, token, req, errors.New(res.Reason))
			StatStorage.AddIosError(1)
			newTokens = append(newTokens, token)
			returnResultList[token] = res
			isError = true
			continue
		}

		if res.Sent() {
			LogPush(SucceededPush, token, req, nil)
			StatStorage.AddIosSuccess(1)
		}
	}

	if isError == true && retryCount < maxRetry {
		retryCount++

		// resend fail token
		req.Tokens = newTokens
		goto Retry
	}

	return &returnResultList,isError
}

// GetAndroidNotification use for define Android notificaiton.
// HTTP Connection Server Reference for Android
// https://developers.google.com/cloud-messaging/http-server-ref
func GetAndroidNotification(req PushNotification) gcm.HttpMessage {
	notification := gcm.HttpMessage{
		To:                    req.To,
		CollapseKey:           req.CollapseKey,
		ContentAvailable:      req.ContentAvailable,
		DelayWhileIdle:        req.DelayWhileIdle,
		TimeToLive:            req.TimeToLive,
		RestrictedPackageName: req.RestrictedPackageName,
		DryRun:                req.DryRun,
	}

	notification.RegistrationIds = req.Tokens

	if len(req.Priority) > 0 && req.Priority == "high" {
		notification.Priority = "high"
	}

	// Add another field
	if len(req.Data) > 0 {
		notification.Data = make(map[string]interface{})
		for k, v := range req.Data {
			notification.Data[k] = v
		}
	}

	notification.Notification = &req.Notification

	// Set request message if body is empty
	if len(req.Message) > 0 {
		notification.Notification.Body = req.Message
	}

	if len(req.Title) > 0 {
		notification.Notification.Title = req.Title
	}

	if len(req.Sound) > 0 {
		notification.Notification.Sound = req.Sound
	}

	return notification
}

func PushToAndroid(req PushNotification) bool {
	var isError bool
	_, isError = PushToAndroidWithErrorResult(req)
	return isError
}


// PushToAndroid provide send notification to Android server.
func PushToAndroidWithErrorResult(req PushNotification) (*map[string]*GoogleResponse,bool) {
	LogAccess.Debug("Start push notification for Android")

	var APIKey string
	var retryCount = 0
	var maxRetry = PushConf.Android.MaxRetry

	if req.Retry > 0 && req.Retry < maxRetry {
		maxRetry = req.Retry
	}

	// check message
	err := CheckMessage(req)

	var returnResultList map[string]*GoogleResponse
	returnResultList = make(map[string]*GoogleResponse)

	if err != nil {
		LogError.Error("request error: " + err.Error())
		return &returnResultList, true
	}

Retry:
	var isError = false
	notification := GetAndroidNotification(req)

	returnResultList = make(map[string]*GoogleResponse)

	if APIKey = PushConf.Android.APIKey; req.APIKey != "" {
		APIKey = req.APIKey
	}

	res, err := gcm.SendHttp(APIKey, notification)

	if err != nil {
		// GCM server error
		LogError.Error("GCM server error: " + err.Error())
		return &returnResultList, true
	}

	LogAccess.Debug(fmt.Sprintf("Android Success count: %d, Failure count: %d", res.Success, res.Failure))
	StatStorage.AddAndroidSuccess(int64(res.Success))
	StatStorage.AddAndroidError(int64(res.Failure))

	var newTokens []string
	for k, result := range res.Results {
		if result.Error != "" {
			isError = true
			newTokens = append(newTokens, req.Tokens[k])
			LogPush(FailedPush, req.Tokens[k], req, errors.New(result.Error))

			response := &GoogleResponse{}
			response.Reason = result.Error

			returnResultList[req.Tokens[k]] = response
			continue
		}

		LogPush(SucceededPush, req.Tokens[k], req, nil)
	}

	if isError == true && retryCount < maxRetry {
		retryCount++

		// resend fail token
		req.Tokens = newTokens
		goto Retry
	}

	return &returnResultList,isError
}
