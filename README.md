# Goexpose

Goexpose is lightweight json api server that maps url path to various tasks.
Goexpose can be used in various scenarios: either make call commands on your servers (or 
farm of servers), or you can use it as monitoring tool.
Builtin tasks are currently:

* shell task - list of shell commands
* http task - call external http request
* info task - information about server
* postgres task - run queries on postgres database
* redis task - run commands on redis
* cassandra task - run cassandra queries
* mysql task - task to run mysql queries
* multi task - run multiple tasks
* filesystem task - serving file(s) from filesystem

I have a plan to implement other tasks with support for: memcache, mongodb, sqlite, file..

All these commands can accepts variables from route (gorilla mux is used).
GOexpose has system for authorization, currently basic (username password) is implemented.
In the future more types of authorization will be implemented.


Lets see example configuration file:

```json
{
    "host": "127.0.0.1",
    "port": 9900,
    "ssl": {
        "cert": "./cert.pem",
        "key": "./key.pem"
    },
    "reload_env": true,
    "endpoints": [{
        "path": "/info",
        "authorizers": ["basic"],
        "methods": {
            "GET": {
                "type": "info",
                "description": "Info task"
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
```

This means that Goexpose will listen on https://127.0.0.1:9900 
"endpoints" is a list of defined endpoints that goexpose responds to.

Configuration:

* host - host that we will listen on
* port - port number
* ssl - ssl settings 
    * cert - cert file
    * key - key file
* reload_env - reload env variables on every request
* endpoints - list of endpoints, config for endpoint:    
    * path - url path
    * authorizers - list of authorizers applied to this endpoint (see Authorizers)
    * methods - dictionary that maps http method to task
        

## Installation:

Run go install 
    
    go install github.com/phonkee/goexpose


## Interpolation:

Goexpose provides various variables from url, query, request.
This data is available in commands to interpolate various strings.
text/template is used and available data is in this structure:
   
```json
{
    "url": {},
    "query": {},
    "request": {
        "method": "",
        "body": ""
    },
    "env": {}
}
```
* env - environment variables
* url - variables from url regular expressions
* query - query values from "query_params"
* request - request vars from goexpose request
    * method - http method from request
    * body - body passed to request

## Query Params:

Goexpose has support to use query parameters. But you have to configure all
params that you want to use. Goexpose gives you possibility to validate params
values with regular expressions and provide default value.
You can provide "query_params" on two levels. You can specify them on endpoint level
and also on task level.

Configuration:

```json
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
```

## Formats:


http task and shell task have possibility to set format of response.
Currently available formats are: "json", "jsonlines", "lines", "text".
Format can be combination of multiple formats. e.g.
    
    "format": "json|jsonlines"

First format that returns result without error will be used.
If "text" is not found in format, it is automatically inserted to the end.

## Tasks:

Tasks can be configured in config["methods"] which is a map[string]TaskConfig - 
http method to task.
Every task config has common part and configuration for given task.
Common configuration is:

```json
{
    "type": "http",
    "authorizers": [],
    "config": {},
    "query_params": {
        "params: [{
            "name": "id",
            "regexp": "^[0-9]+$",
            "default": "0"
        }],
        "return_params": true
    }
}
```

* type - type of task.
* authorizers - list of authorizers for given endpoint (see Authorizers)
* config - configuration for given task type (will describe later in each task)
* query_params - query params (see Query Params)
* return_params - whether goexpose should return those params in response


### HttpTask:

Http task is task that can do external request. Task configuration is following:

```json
{
    "type": "http",
    "config": {
        "single_result": 0,
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
```

Configuration:

* urls - list of url configurations
    * url - url to send request to, url is interpolated (see Interpolation)
    * method - request to url will not have the same method as request to goexpose, given method value
        will be used
    * format - format of response, if no format is given goexpose will try to read Content-Type, if application/json
        (see Formats)
    * return_headers - whether to return response headers from url response to goexpose response
    * post_body - if goexpose should post body of goexpose request to url
* single_result - only that result will be returned (unwrapped from array)

### ShellTask:


ShellTask is task that is able to run shell commands on target server. Every command
is interpolated (see Interpolation)

**Warning!!!** Use appropriate regular expressions for inputs so you don't expose your server
to shell injection.
Example:

```json
{
    "type": "shell",
    "config": {
        "env": {
            "key": "value"
        },
        "shell": "/bin/bash",
        "commands": [{
            "command": "echo \"{{.url.id}}\"",
            "chdir": "/tmp",
            "format": "json",
            "return_command": true
        }]
    }
}
```

Configuration:

* env - custom environment variables
* shell - shell to run command with
* commands - list of commands to be called:
    * command - shell command to be run, interpolated (see Interpolation)
    * chdir - change directory before run command
    * format - format of the response (see Formats)
    * return_command - whether to return command in response
* single_result - index which command will be "unwrapped" from result array

### InfoTask:


Info task returns information about goexpose. In result you can find version of goexpose and also
all registered tasks with info. Task info has no configuration.

### PostgresTask:

Run queries on postgres database. Configuration for postgres task:

```json
{
    "type": "postgres",
    "config": {
        "return_queries": true,
        "queries": [{
            "url": "postgres://username:password@localhost/database",
            "query": "SELECT * FROM product WHERE id = $1",
            "args": [
                "{{.url.id}}"
            ]
        }]
    }
}
```

Configuration:

* return_queries - whether queries with args should be added 
* queries - list of queries
    * url - postgres url (passed to sql.Open, refer to https://github.com/lib/pq), interpolated (see Interpolation)
    * methods - allowed methods, if not specified all methods are allowed
    * query - sql query with placeholders $1, $2 ... (query is not interpolated!!!)
    * args - list of arguments to query - all queries are interpolated (see Interpolation).
* single_result - index which query will be "unwrapped" from result array

### RedisTask:

Task that can run multiple commands on redis. Example:

```json
{
    "type": "redis",
    "config": {
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
        }]
    }
}
```
    
Configuration:
  
* address - address to connect to (see http://godoc.org/github.com/garyburd/redigo/redis#Dial)
    Default: ":6379", interpolated (see Interpolation)
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
* single_result - index which query will be "unwrapped" from result array

### CassandraTask:

Run cassandra queries task. Example:

```json
{
    "type": "cassandra",
    "config": {
        "return_queries": true,
        "queries": [{
            "query": "SELECT * from user WHERE id = ?",
            "args": [
                "{{.url.id}}"
            ],
            "cluster": [
                "192.168.1.1",
                "192.168.1.2"
            ],
            "keyspace": "keyspace"
        }]
    }
}
```

Configuration:

* return_queries - whether to return query, args in response
* queries - list of queries configurations, query configuration:
    * query - query with placeholders
    * args - arguments to query which are interpolated (See interpolation)
    * cluster - list of hosts in cluster, all args are interpolated (see Interpolation)
    * keyspace - keyspace to use, interpolated (see Interpolation)
* single_result - index which query will be "unwrapped" from result array


### MySQLTask:

Run mysql queries. Example:

```json
{
    "type": "mysql",
    "config": {
        "return_queries": true,
        "queries": [{
            "url": "user:password@localhost/dbname",
            "query": "SELECT * FROM auth_user WHERE id = ?",
            "args": [
                "{{.url.id}}"
            ]
        }]
    }
}
```

Configuration:

* return_queries - whether to return query with args to response
* queries - list of queries, query config:
    * url - url to connect to (refer to https://github.com/go-sql-driver/mysql), interpolated (see Interpolation)
    * query - query with placeholders
    * args - list of arguments, every argument will be interpolated (see Interpolation)
* single_result - index which query will be "unwrapped" from result array

### MultiTask:

Multi task gives possibility to run multiple tasks in one task. These task can be any tasks (except of embedded multi task).

```json
{
    "type": "multi",
    "config": {
        "single_result": 0,
        "tasks": [{
            "type": "http",
            "config": {
            "single_result": 0,
            "urls": [{
                "url": "http://www.google.com"
            }]
        }
    }]        
}
```

Configuration:

* single_result - index which task will be "unwrapped" from result array
* tasks - list of tasks (these embedded tasks does not support authorizers)


### FilesystemTask:

Filesystem task is simple yet powerful task. It can be configured to serve single file or serve all files in directory
with optional index page for directories.

In following example we serve only one file on url /file/some. The output will be json with base encoded file content.

```json
{
    "path": "/file/some",
    "methods": {
        "GET": {
            "type": "filesystem",
            "config": {
                "file": "/tmp/file"
            }
        }
    }
}
```

In next example we will serve files in directory and provide index page for directories and also give possibility to 
return raw file as response.

```json
{
    "path": "/static/{file:.+}",
    "methods": {
        "GET": {
            "query_params": {
                "params": [{
                    "name": "output",
                    "regexp": "^raw$",
                    "default": ""
                }],
            },                
            "config": {
                "file": "{{.url.file}}",
                "output": "{{.query.output}}",
                "directory": "/tmp",
                "index": true
            }
        }
    }
}
```

Configuration:

* file - file to serve (interpolated)
* directory - base directory (interpolated)
* output - type of the output (interpolated)
    * "raw" - returns raw file contents, otherwise it's wrapped to json
* index - whether to serve index endpoint for directory


## Authorizers:

Types of authentication ( I know it's silly name..)
First you have to define your authorizers in top level "authorizers" and then you can use
them in your tasks defined by name. e.g.:


```json
{
    "endpoints": [{
        "path": "/info",
        "authorizers": ["username_pass"],
        "methods": {
            "GET": {
                "type": "info",
            }
        }
    }],
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
```

You can set your authorizers in endpoint configuration, or you can set in every task for fine tuned
configuration.

### Basic

Support for basic authentication.

```json
{
    "type": "basic",
    "config": {
        "username": "hello",
        "password": "world"
    }
}
```


# Example:

in folder example/ there is complete example for couple of tasks.
You can find example [here!](example/config.json) or [yaml!](example/config.yaml)

@TODO:
Add tasks for: sqlite, memcached, mongodb
  
## Author:
phonkee
