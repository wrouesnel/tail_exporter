[![Build Status](https://travis-ci.org/wrouesnel/tail_exporter.svg)](https://travis-ci.org/wrouesnel/tail_exporter)

# Tail Exporter

This is a prometheus exporter in a similar vein as mtail, but implemented to
be able to read on TCP sockets, and with a YAML format. It is also capable of
reading from log files or pipes.

It links against the libpcre library for faster regex'ing.

# Configuration File

The configuration file is based on YAML. Prometheus metrics are required to
have a consistent set of labels, but metrics may be repeated in the config file
to allow multiple regexes to populate different timeseries.
