# Apigee client library

[![Go Reference](https://pkg.go.dev/badge/github.com/bstraehle/apigee-client-go.svg)](https://pkg.go.dev/github.com/bstraehle/apigee-client-go) [![Go Report Card](https://goreportcard.com/badge/github.com/bstraehle/apigee-client-go)](https://goreportcard.com/report/github.com/bstraehle/apigee-client-go) [![GitHub release (latest SemVer including pre-releases)](https://img.shields.io/github/v/release/bstraehle/apigee-client-go?color=red&include_prereleases&sort=semver)](https://github.com/bstraehle/apigee-client-go/releases)

A Go library for Apigee to create and delete a consumer key and consumer secret. Purpose-built for the [Vault Apigee secrets engine](https://github.com/bstraehle/vault-plugin-secrets-apigee).

## Table of Contents

1. [Prerequisites](#1-prerequisites)
2. [Configure Access](#2-configure-access)
3. [Configure Environment](#3-configure-environment)
4. [Build/Test](#4-buildtest)

## 1. Prerequisites

- [Git](https://git-scm.com/downloads) (Optional)
- [Go](https://go.dev/dl/) (Optional)
- [Apigee X](https://cloud.google.com/apigee/docs/) and [Google Cloud CLI](https://cloud.google.com/sdk/docs/install) or
- [Apigee Edge](https://docs.apigee.com/)

## 2. Configure Access

```
gcloud auth login
export APIGEE_OAUTH_TOKEN=$(gcloud auth print-access-token)
```

or

```
export APIGEE_USERNAME=<APIGEE_USERNAME>
export APIGEE_PASSWORD=<APIGEE_PASSWORD>
```

## 3. Configure Environment

```
export APIGEE_HOST=<APIGEE_HOST>
export APIGEE_ORG_NAME=<APIGEE_ORG_NAME>
export APIGEE_DEVELOPER_EMAIL=<APIGEE_DEVELOPER_EMAIL>
export APIGEE_APP_NAME=<APIGEE_APP_NAME>
export APIGEE_API_PRODUCTS=["<APIGEE_API_PRODUCT>"]
```

## 4. Build/Test

```
git clone https://github.com/bstraehle/apigee-client-go.git
cd apigee-client-go

go build client.go credentials.go credentials_test.go
go test -v
```
