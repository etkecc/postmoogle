// Copyright (c) 2024 Sumner Evans
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package exhttp

import "net/http"

type ErrorBodyGenerators struct {
	NotFound         func() []byte
	MethodNotAllowed func() []byte
}

func HandleErrors(next http.Handler, gen ErrorBodyGenerators) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&bodyOverrider{
			ResponseWriter:                      w,
			statusNotFoundBodyGenerator:         gen.NotFound,
			statusMethodNotAllowedBodyGenerator: gen.MethodNotAllowed,
		}, r)
	})
}

type bodyOverrider struct {
	http.ResponseWriter

	code     int
	override bool

	statusNotFoundBodyGenerator         func() []byte
	statusMethodNotAllowedBodyGenerator func() []byte
}

var _ http.ResponseWriter = (*bodyOverrider)(nil)

func (b *bodyOverrider) WriteHeader(code int) {
	if b.Header().Get("Content-Type") == "text/plain; charset=utf-8" {
		b.Header().Set("Content-Type", "application/json")

		b.override = true
	}

	b.code = code
	b.ResponseWriter.WriteHeader(code)
}

func (b *bodyOverrider) Write(body []byte) (int, error) {
	if b.override {
		switch b.code {
		case http.StatusNotFound:
			if b.statusNotFoundBodyGenerator != nil {
				body = b.statusNotFoundBodyGenerator()
			}
		case http.StatusMethodNotAllowed:
			if b.statusMethodNotAllowedBodyGenerator != nil {
				body = b.statusMethodNotAllowedBodyGenerator()
			}
		}
	}

	return b.ResponseWriter.Write(body)
}
