# Dev module
The *Dev* module defines the interface of the service context.
It also includes the definition of the development context. 
All contexts must have the interface defined in this module.

> **Warning**
> 
> * Tests sometimes fail due to downloading source code.
> * Tests fail due to firewall

## Context

The **context** is the abstracted accessor to the hosting environment.

During the development, the service is hosted on the laptop.
In production, a service maybe deployed to the cloud.
On top of that, the service maybe containerized or stored as a binary.

All hosting options have different settings and features.
Especially when it comes to service discovery and configuration. 
By abstracting the environment, the application is decoupled from the hosting.
For each hosting provider, the context shall use the most optimized solutions.

The distributed systems could be hosted on multiple servers.
**The context abstracts the multiple machines**.

Anything that service needs from the hosting environment comes from the context.
Whether it's coming from the same server or remote server is not the worry of the service.

> **todo**
> 
> [Martin Fowler](https://martinfowler.com/) said this.
> For remote services the API must be coarsely grained.
> 
> Then, the AI must be able to create a coarsely grained API from smaller APIs.
> In order to solve it: 
> - the proxy could be set using inproc protocol.
> - the proxy can have its own rule set.

For now, the **context** has.
* The configuration engine.
* The dependency manager.

> Not implemented yet, but assume the 
> file operations, networking and logging are also included in the context.

> **Todo**
> 
> Add a network feature that allocates an address for the service.

## Configuration engine
The configuration is not the part of the code.
Thus, it comes from the hosting environment.

The configurations maybe passed in a different ways.
As a file in `json`, `ini` or `yaml` formats.
By third-party provider. For example, with [etcd](https://etcd.io/). 
The cloud providers also have their own configurations.

**The task of the engine is to use the best solution with minimum setup in each environment.**

*More information about a configuration engine is available on [config-lib](https://github.com/sds-framework/config-lib).*

## Dependency manager

> **Todo**
> 
> **Problem**: Dep services are attached to the current process.
> Closing the current process will close the running dependencies.
> 
> **Solution**:
> Spawn the child processes as the service.
> Use [kardianos/service](https://pkg.go.dev/github.com/kardianos/service) package.
> 
> Useful links:
> - [How to programmamitcally start a linux console on *Reddit*](https://www.reddit.com/r/golang/comments/12qhzfc/how_to_programmatically_start_a_linux_console/)
> - [How to build a service in Golang on *Medium*](https://levelup.gitconnected.com/how-to-build-a-service-in-golang-9af2b7ed92a7)
> - [Forking a Deamon process on Unix on *O'Reilly*](https://www.oreilly.com/library/view/python-cookbook/0596001673/ch06s08.html)
> - [Web server using *kardianos/service* on *sachinsu* gist](https://gist.github.com/sachinsu/78fc6a3b4767df0d45de42f8b140dd4a)
> - [Cannot start application as a Windows service on *StackOverflow*](https://stackoverflow.com/questions/35605238/cannot-start-a-go-application-exe-as-a-windows-services)
> - [Detaching currently running go golang process on *Reddit*](https://www.reddit.com/r/golang/comments/vrkhsn/detaching_from_the_currently_running_golang/)
> - [Start a process and detach it in go on *StackOverflow*](https://stackoverflow.com/questions/23031752/start-a-process-in-go-and-detach-from-it)
> - [Can I run a golang application as a background service without nohup on *StackOverflow*](https://stackoverflow.com/questions/23031752/start-a-process-in-go-and-detach-from-it)

The distributed systems must run all the services together.
The services must be reliable and interconnected.

There are two approaches for managing distributed systems.

The first way is utilizing a separated tool.
This tool is called an *orchestrator.*
The popular orchestrators include *kubernetes*, *docker compose*.

The other way is to make an application self-orchestrating.
Each service is responsible for reliability and discovery without an orchestrator.
In my career as a coder, I did not encounter anyone who is doing by this approach.
Because, self-orchestration adds a design complexity at the module level.
Also, the code gets more complex as it includes a business logic and orchestration.

Using the orchestration tool, at the coding stage, a developer could focus on the business logic.
Without worrying about orchestrating each part.
However, the overall architecture is complex and requires more hardware resources.
It also adds a centralization which creates a single point of failure.
Self-orchestration has more benefits that over-comes the orchestration tools.
It reduces the parts that service depends on. 
Finally, it makes the code-testable for inter-connection which is the hard task, actually.

SDS is the only framework that chose the second approach.
By moving the orchestration part to the framework, the programmer doesn't have to write the tedious part.

### Services relationships

To write the multithreading applications, there are programming languages with concurrency.
The best example is [Erlang](https://www.erlang.org/).
In this approach, the application could be imagined as a tree of the processes.
A main thread spawns the child threads, sets up a message bus.
If the parent process dies, then all child processes die along with it.
If the child process dies, then the parent may respawn it again.

SDS framework works in the same way.
There is a primary service with the application logic. 
Then, there are additional services that do a side work.
The primary service is spawning additional services.

For example, the primary service could be an API.
While the additional service could be a database driver.

In a more complex application, there are multiple layers of the services.
The additional services may depend on other services as well.

In the example above, the database driver may depend on the authentication service.

In the SDS framework, a service manages the dependencies that have direct messaging with it.
If the dependency has an own set of dependencies, then it's none of primary service businesses.

Again, refer to the [config-lib](https://github.com/sds-framework/config-lib)

---

# Dev Context
Which means it's in the current machine.

The dependencies are including the extensions and proxies.

How it works?

The orchestra is set up. It checks the folder. And if they are not existing, it will create them.
>> dev.Run(orchestra)

Then let's work on the extension.
User is passing an extension url.
The service is checking whether it exists in the data or not.
If the service exists, it gets the yaml. 
And return the config.

If the service doesn't exist, it checks whether the service exists in the bin.
If it exists, then it runs it with --build-config.

Then, if the service doesn't exist in the bin, it checks the source.
If the source exists, then it will call `go build`.
Then call bin file with the generated files.

Lastly, if a source doesn't exist, it will download the files from the repository using go-git.
Then we build the binary.
We generate config.

Lastly, the service.Run() will make sure that all binaries exist.
If not, then it will create them.

-----------------------------------------------
The running the application will do the following.
It checks the port of proxies is in use.
If it's not, then it will call a run.

Then it will call itself.

The service will have a command to "shutdown" contexts. As well as "rebuild"

----

# Dep Manager
Dep manager has sockets that work with the service.
The reason is to reduce the logic in the service. 
Changing the dependencies is done by context.
The dev manager without a socket will be synchronous.
That means the service will be blocked until it finishes the dep process.

Now, the context will keep track of all dependencies and their management.