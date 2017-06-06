[![Build Status](https://travis-ci.org/wrouesnel/tail_exporter.svg)](https://travis-ci.org/wrouesnel/tail_exporter)
[![Coverage Status](https://coveralls.io/repos/github/wrouesnel/tail_exporter/badge.svg?branch=master)](https://coveralls.io/github/wrouesnel/tail_exporter?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/wrouesnel/tail_exporter)](https://goreportcard.com/report/github.com/wrouesnel/tail_exporter)

# Tail Exporter

This is a prometheus exporter in a similar vein as mtail, but implemented to
be able to read on TCP sockets, and with a YAML format. It is also capable of
reading from log files or pipes.

It links against the libpcre library for faster regex'ing.

# Configuration File

The configuration file is based on YAML. Prometheus metrics are required to
have a consistent set of labels, but metrics may be repeated in the config file
to allow multiple regexes to populate different timeseries.

## Example
Counting mail processing stages from exim:
```yaml
metric_configs:
- name: exim_mail_count
  help: counter of mail processing results
  type: counter
  regex: '^METRICS: uuid=(\S+) result=(\S+)'
  labels:
  - name: uuid
    value: $1
  - name: result
    value: $2
  value: increment
  timeout: 15m
```
