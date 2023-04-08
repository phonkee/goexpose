# Tasks

## Convert to zap logger

This is necessary to get structured logging

## move tasks to package task

separate package for tasks

## cli urfave/cli

Introduce commands and subcommands

## better error reporting with validating config (also separate subtask)

Use go-response package

Use that instead of homebaked solution

## have main func to be called, so someone can register its own tasks

When someone wants to customize tasks, they can register their own tasks.

## unify errors
no more `error_code`