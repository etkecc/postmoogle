// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package event

import (
	"maunium.net/go/mautrix/id"
)

type MessageStatusReason string

const (
	MessageStatusGenericError      MessageStatusReason = "m.event_not_handled"
	MessageStatusUnsupported       MessageStatusReason = "com.beeper.unsupported_event"
	MessageStatusUndecryptable     MessageStatusReason = "com.beeper.undecryptable_event"
	MessageStatusTooOld            MessageStatusReason = "m.event_too_old"
	MessageStatusNetworkError      MessageStatusReason = "m.foreign_network_error"
	MessageStatusNoPermission      MessageStatusReason = "m.no_permission"
	MessageStatusBridgeUnavailable MessageStatusReason = "m.bridge_unavailable"
)

type MessageStatus string

const (
	MessageStatusSuccess   MessageStatus = "SUCCESS"
	MessageStatusPending   MessageStatus = "PENDING"
	MessageStatusRetriable MessageStatus = "FAIL_RETRIABLE"
	MessageStatusFail      MessageStatus = "FAIL_PERMANENT"
)

type BeeperMessageStatusEventContent struct {
	Network   string              `json:"network,omitempty"`
	RelatesTo RelatesTo           `json:"m.relates_to"`
	Status    MessageStatus       `json:"status"`
	Reason    MessageStatusReason `json:"reason,omitempty"`
	// Deprecated: clients were showing this to users even though they aren't supposed to.
	// Use InternalError for error messages that should be included in bug reports, but not shown in the UI.
	Error         string `json:"error,omitempty"`
	InternalError string `json:"internal_error,omitempty"`
	Message       string `json:"message,omitempty"`

	LastRetry id.EventID `json:"last_retry,omitempty"`

	MutateEventKey string `json:"mutate_event_key,omitempty"`

	// Indicates the set of users to whom the event was delivered. If nil, then
	// the client should not expect delivered status at any later point. If not
	// nil (even if empty), this field indicates which users the event was
	// delivered to.
	DeliveredToUsers *[]id.UserID `json:"delivered_to_users,omitempty"`
}

type BeeperRetryMetadata struct {
	OriginalEventID id.EventID `json:"original_event_id"`
	RetryCount      int        `json:"retry_count"`
	// last_retry is also present, but not used by bridges
}

type BeeperRoomKeyAckEventContent struct {
	RoomID            id.RoomID    `json:"room_id"`
	SessionID         id.SessionID `json:"session_id"`
	FirstMessageIndex int          `json:"first_message_index"`
}

type LinkPreview struct {
	CanonicalURL string `json:"og:url,omitempty"`
	Title        string `json:"og:title,omitempty"`
	Type         string `json:"og:type,omitempty"`
	Description  string `json:"og:description,omitempty"`

	ImageURL id.ContentURIString `json:"og:image,omitempty"`

	ImageSize   int    `json:"matrix:image:size,omitempty"`
	ImageWidth  int    `json:"og:image:width,omitempty"`
	ImageHeight int    `json:"og:image:height,omitempty"`
	ImageType   string `json:"og:image:type,omitempty"`
}

// BeeperLinkPreview contains the data for a bundled URL preview as specified in MSC4095
//
// https://github.com/matrix-org/matrix-spec-proposals/pull/4095
type BeeperLinkPreview struct {
	LinkPreview

	MatchedURL      string             `json:"matched_url,omitempty"`
	ImageEncryption *EncryptedFileInfo `json:"beeper:image:encryption,omitempty"`
}

type BeeperProfileExtra struct {
	RemoteID     string   `json:"com.beeper.bridge.remote_id,omitempty"`
	Identifiers  []string `json:"com.beeper.bridge.identifiers,omitempty"`
	Service      string   `json:"com.beeper.bridge.service,omitempty"`
	Network      string   `json:"com.beeper.bridge.network,omitempty"`
	IsBridgeBot  bool     `json:"com.beeper.bridge.is_bridge_bot,omitempty"`
	IsNetworkBot bool     `json:"com.beeper.bridge.is_network_bot,omitempty"`
}

type BeeperPerMessageProfile struct {
	ID          string               `json:"id"`
	Displayname string               `json:"displayname,omitempty"`
	AvatarURL   *id.ContentURIString `json:"avatar_url,omitempty"`
	AvatarFile  *EncryptedFileInfo   `json:"avatar_file,omitempty"`
}
