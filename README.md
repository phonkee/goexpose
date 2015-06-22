Goexpose
-------

Goexpose is lightweitght json api server that maps url path to various tasks.
Goexpose can be used in various scenarios: either make call commands on your servers (or 
farm of servers), or you can use it as monitoring tool.
Builtin tasks are currently:

* shell task - list of shell commands
* http task - call external http request
* info task - information about server
* postgres task - run queries on postgres database
* redis task - run commands on redis

I have a plan to implement other tasks with support for: memcache,
mysql, cassandra..

All these commands can accepts variables from route (gorilla mux is used).
GOexpose has system for authorization, currently basic (username password) is implemented.
In the future more types of authorization will be implemented.


Lets see example configuration file:

    {
        "host": "127.0.0.1",
        "port": 9900,
        "ssl": {
            "cert": "./cert.pem",
            "key": "./key.pem"
        },
        "endpoints": [{
            "path": "/info",
            "authorizers": ["basic"],
            "methods": {
                "GET": {
                    "type": "info",
                }
            }
        }],
        "authorizers": {
            "basic": {
                "type": "basic",
                "config": {
                    "username": "hello",
                    "password": "world"
                }
            }
        }
    }

This means that Goexpose will listen on https://127.0.0.1:9900 
"endpoints" is a list of defined endpoints that goexpose responds to.

Configuration:
* host - host that we will listen on
* port - port number
* ssl - ssl settings 
    * cert - cert file
    * key - key file
* endpoints - list of endpoints, config for endpoint:    
    * path - url path
    * authorizers - list of authorizers applied to this endpoint (see Authorizers)
    * methods - dictionary that maps http method to task
        
Installation:
-------------

Run go install 
    
    go install github.com/phonkee/goexpose


Interpolation:
--------------

Goexpose provides various variables from url, query, request.
This data is available in commands to interpolate various strings.
text/template is used and available data is in this structure:
    
    {
        "url": {},
        "query": {},
        "request": {
            "method": "",
            "body": ""
        }
    }

* url - variables from url regular expressions
* query - query values from "query_params"
* request - request vars from goexpose request
    * method - http method from request
    * body - body passed to request

Query Params:
-------------

Goexpose has support to use query parameters. But you have to configure all
params that you want to use. Goexpose gives you possibility to validate params
values with regular expressions and provide default value.
You can provide "query_params" on two levels. You can specify them on endpoint level
and also on task level.

Configuration:

    {
        "query_params": {
            "return_params": true,
            "params": [{
                "name": "page",
                "regexp": "^[0-9]+$",
                "default": "0"
            }, {
                "name": "limit",
                "regexp": "^[0-9]+$",
                "default": "10"
            }]
        }
    }


Tasks:
=====

Tasks can be configured in config["methods"] which is a map[string]TaskConfig - 
http method to task.
Every task config has common part and configuration for given task.
Common configuration is:

    {
        "type": "http",
        "authorizers": []
        "methods": ["GET"],
        "config": {},
        "query_params": [
            {
                "name": "id",
                "regexp": "^[0-9]+$",
                "default": "0"
            }
        ],
        "return_params": true
    }

* type - type of task (currently supported tasks "shell", "http", "info").
         I will explain all tasks later
* authorizers - list of authorizers for given endpoint (see Authorizers)
* methods - supported http methods (uppercased)
* config - configuration for given task type (will describe later in each task)
* query_params - list of query parameters accepted from url query. All used query params
                 must be defined here
                 * name - name of parameter ?xxx=yyy name is xxx
                 * regexp - regular expression to accept parameter
                 * default - default value if value is not presented or regexp match failed
                 All query params can be used in tasks that interpolate commands
                 in namespace "query" so if we define "xxx" param in 
                 http task in url we can use it "http://www.google.com/?q={{.query.xxx}}
* return_params - whether goexpose should return those params in response


HttpTask:
---------

Http task is task that can do external request. Task configuration is following:

    {
        "type": "http",
        "path": "/some/path",
        {
            "urls": [{
                "url": "http://127.0.0.1:8000/{{.url.id}}",
                "post_body": false,
                "format": "json",
                "return_headers": false
            }, {
                "url": "http://127.0.0.1:8000/{{.url.id}}",
                "method": "PUT",
                "post_body": false,
                "format": "json",
                "return_headers": false,
                "post_body": true,
            }]
        }
    
    }

Url config:

* urls - list of url configurations
    
    * url - url to send request to, url is interpolated (see Interpolation)
    * method - request to url will not have the same method as request to goexpose, given method value
        will be used
    * format - format of response, if no format is given goexpose will try to read Content-Type, if application/json
        is found format will be set to json, if format is given will be used.
    * return_headers - whether to return response headers from url response to goexpose response
    * post_body - if goexpose should post body of goexpose request to url

ShellTask:
----------

ShellTask is task that is able to run shell commands on target server. Every command
is interpolated (see Interpolation)

**Warning!!!** Use appropriate regular expressions for inputs so you don't expose your server
to shell injection.
Example:

    {
        "env": {
            "key": "value"
        },
        "shell": "/bin/bash",
        "commands": [
            {
                "command": "echo \"{{.url.id}}\"",
                "chdir": "/tmp",
                "format": "json",
                "return_command": true
            }
        ]
    }

Configuration:
* env - custom environment variables
* shell - shell to run command with
* commands - list of commands to be called:
    Command has these configuration:
        * command - shell command to be run, interpolated (see Interpolation)
        * chdir - change directory before run command
        * format - format of the response
        * return_command - whether to return command in response
    
InfoTask:
---------

Info task returns information about goexpose. In result you can find version of goexpose and also
all registered tasks with info. Task info has no configuration.

PostgresTask:
-------------

Run queries on postgres database. Configuration for postgres task:

    {
        "url": "postgres://username:password@localhost/database",
        "return_queries": true,
        "queries": [{
            "query": "SELECT * FROM product WHERE id = $1",
            "args": [
                "{{.url.id}}"
            ]
        }, {
            "query": "DELETE FROM product WHERE id = $1",
            "args": [
                "{{.url.id}}"
            ]
        }]
    }

Configuration:
* url - postgres url (passed to sql.Open)
* return_queries - whether queries with args should be added 
* queries - list of queries
    * methods - allowed methods, if not specified all methods are allowed
    * query - sql query with placeholders $1, $2 ... (query is not interpolated!!!)
    * args - list of arguments to query - all queries are interpolated (see Interpolation).

RedisTask:
----------

Task that can run multiple commands on redis. Example:

    {
        "address": "127.0.0.1:6379",
        "network": "tcp",
        "database": 1,
        "return_queries": true,
        "queries": [{
            "command": "GET",
            "args": [
                "product:{{.url.id}}"
            ],
            "type": "string"
        }
        ]
    }

Config:
  
* address - address to connect to (see http://godoc.org/github.com/garyburd/redigo/redis#Dial)
    Default: ":6379"
* network - network (see http://godoc.org/github.com/garyburd/redigo/redis#Dial)
    Default: "tcp"
* database - database number
    Default: 1
* return_queries - whether to return queries in response
* queries - list of queries settings
    * command - redis command
    * args - arguments passed to redis command, all arguments are interpolated (see Interpolation)
    * type - type of return value. Possible values are:
        * bool
        * float64
        * int
        * int64
        * ints - list of integers
        * string - default
        * strings - list of strings
        * uint64
        * values
        * stringmap - map[string]string

Authorizers:
------------

Types of authentication ( I know it's silly name..)
First you have to define your authorizers in top level "authorizers" and then you can use
them in your tasks defined by name. e.g.:

    {
        "endpoints": [{
            "path": "/info",
            "authorizers": ["username_pass"],
            "methods": {
                "GET": {
                    "type": "info",
                }
            }
        }]
        "authorizers": {
            "username_pass": {
                "type": "basic",
                "config": {
                    "username": "hello",
                    "password": "world"
                }
            }
        }
    }

You can set your authorizers in endpoint configuration, or you can set in every task for fine tuned
configuration.
Currently there is only "basic" authorizer implemented, but in the future I plan to implement
other types such as: postgres, shell, mysql..

Example:
--------

in folder example/ there is complete example for all tasks.




TODO:
add tests
  
Author:
-------

phonkee

