# yama
Simple library to make it easier to manage the death of your application.

[![Build Status](https://travis-ci.org/livetribe/yama.svg?branch=master)](https://travis-ci.org/livetribe/yama) 
[![Go Report Card](https://goreportcard.com/badge/github.com/livetribe/yama)](https://goreportcard.com/report/github.com/livetribe/yama) 
[![Documentation](https://godoc.org/github.com/livetribe/yama?status.svg)](http://godoc.org/github.com/livetribe/yama) 
[![Coverage Status](https://coveralls.io/repos/github/livetribe/yama/badge.svg)](https://coveralls.io/github/livetribe/yama)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/livetribe/yama.svg?style=social)](https://github.com/livetribe/yama/tags)

![Image of Yama](https://github.com/livetribe/yama/raw/master/img/yama.jpg)

Yama provides a signal watcher that can be used to shutdown an application.

A signal watcher can be constructed to watch any number of signals and will
call any number of registered `io.Closer` instances, when such signals occur; the
results of calling `Close()` on the registered instances are ignored.

	watcher := yama.NewWatcher(
		yama.WatchingSignals(syscall.SIGINT, syscall.SIGTERM),
		yama.WithTimeout(2*time.Second),
		yama.WithClosers(server))

An application can wait fir the completion of the `Closer` notifications by
calling the blocking method, `Wait()`.

    watcher.Wait()

Here, the caller will be blocked until one of the signals occur and all the
`Closer` notifications have either completed or two seconds have elapsed since
the start of `Closer` notifications; the timeout is set above by passing
`yama.WithTimeout()`.  Subsequent signals will not trigger `Closer` notifications.

The application can programmatically trigger `Closer` notifications by calling

    watcher.Close()

If this is done, subsequent signals will not trigger `Closer` notifications.

There are a few helper methods, `FnAsCloser()` and `ErrValFnAsCloser()`, that can
be used to wrap simple functions and functions that can return an error,
respectively, into instances that implement `io.Closer`.
