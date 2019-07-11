# logstat

Simple console based tool that monitors access log file 
and prints summary statistics on the traffic as a whole.

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

Check for race conditions
> make race

### Without Make
> go build -o logstat

## Features and architecture
The whole application is constructed from small, reusable components.
All component are properly documented, so you can rely on documentation there.
In general application composed in such fashion that is should tolerate 
failures of other components or missed files or unexpected formatting of logs etc.
All potentially heavy loaded components like file reader, log parser, records aggregator
implemented in "near zero allocation" fashion.
From my test the only allocations present are in alert aggregator and view 
components that shouldn't be under pressure.

High level structure of components:
![Components Diagram](doc/mermaid-component-diagram.svg)
