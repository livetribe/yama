# yama

A compile-time lifecycle orchestration framework: it derives application
startup/quiesce/shutdown ordering from a Google Wire dependency graph and
generates the orchestration code, rather than building a runtime engine that
interprets one.

[![Build Status](https://github.com/livetribe/yama/actions/workflows/ci.yml/badge.svg)](https://github.com/livetribe/yama/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/livetribe/yama)](https://goreportcard.com/report/github.com/livetribe/yama)
[![Documentation](https://godoc.org/l7e.io/yama/v2?status.svg)](http://godoc.org/l7e.io/yama/v2)
[![Coverage Status](https://coveralls.io/repos/github/livetribe/yama/badge.svg?branch=v2)](https://coveralls.io/github/livetribe/yama?branch=v2)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

![Image of Yama](https://github.com/livetribe/yama/raw/master/img/yama.jpg)

## Status

This is v2, a green-field rewrite; it shares only a name and a repository
with the earlier `v0.x` signal-watcher (see
[ADR-011](docs/adr/ADR-011-v1-v2-disposition.md)). v2 is under active
construction and exports no public API yet.

For the design, see:

* [`docs/PRD.md`](docs/PRD.md) — product requirements.
* [`docs/adr/`](docs/adr/) — the accepted architecture decision records.
* [`docs/Architecture.md`](docs/Architecture.md) — the resolved architecture,
  including the Public API Reference.
* [`implementation_plan_claude.md`](implementation_plan_claude.md) — the
  phased implementation plan.
