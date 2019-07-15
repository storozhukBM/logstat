# logstat

A simple console-based tool that monitors access log file 
and prints summary statistics on the traffic as a whole.

IMPORTANT: This tool reporting and alerting logic is entirely based on timestamps from log records, 
to eliminate dependency on I/O speed and enable possibility log-playback.

Currently supported log formats:
* [W3C](https://www.w3.org/Daemon/User/Config/Logging.html) 

## Usage
>logstat -fileName /tmp/access.log

`/tmp/access.log` is a default, so you can omit it
 
Max requests rate for traffic alert can be configured:
 >logstat -trafficAlertMaxTrafficInReqPerSecond 250
 
For alerts only use:
>logstat | grep 'ALERT\|RESOLVED'

For traffic reports only use:
>logstat | grep '^|'

For other configs check help:
>logstat -h

## Build and Development
Add this project to your `$GOPATH`.

### Using Make
Build using Makefile
> make build

Run tests
> make test

Check coverage
> make coverage

Check for race conditions (this can take a while)
> make race

### Without Make
> go build -o logstat

## Features and architecture
The whole application is constructed from small, reusable components.
All component are properly documented, so you can rely on documentation there.
In general, application composed in such fashion that should tolerate 
failures of other components or missing files or unexpected formatting of logs etc.
All potentially heavy loaded components like file reader, log parser, records aggregator
implemented in "near-zero allocation" fashion.
From my tests, the only allocations present are in alert aggregator and view 
components that shouldn't be under pressure.

## Performance
On my machine, this tool is capable of processing ~200 [MB] of logs per second on one core, 
which is ~2.5M [req/sec] (typical size of one line is ~80 bytes).
pprof shows that I'm actually bounded by disk throughput, but of course some further optimizations possible.

## High level structure of components:
![Components Diagram](doc/mermaid-component-diagram-V01.svg)
