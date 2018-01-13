gobearmon
=========

gobearmon is a **distributed uptime monitoring system**.

The basic uptime monitoring concept is simple:

* Checks, contacts, and alerts are configured in a MySQL database.
* A check specifies an action verifying that a service is online, e.g. pinging an IP address, opening a TCP connection, or fetching a webpage.
* A contact specifies a notification action, e.g. sending an e-mail or SMS message.
* An alert links checks with contacts, e.g. contact Alice and Bob but not Charlie when check X goes offline.

gobearmon is distributed for two reasons. First, to ensure that monitoring continues even if a monitoring node fails. Second, so that notifications are sent only when the check fails on multiple nodes; for example, if a transient network issue interrupts communication between node A and service X, but node B still sees service X is online, then we may not want to send a notification. The distributed system model is:

* There are N workers and 1 viewserver.
* At any given time, one worker acts as the controller (master). The viewserver monitors the controller; if the viewserver sees that the former controller is offline, it selects a new controller. Workers query the viewserver to find out who the current controller is.
* The controller coordinates monitoring among the workers.

Checks
------

These checks and configuration options are supported:

* HTTP: can configure timeout, headers, request method/body; can verify the status code or verify that a substring appears in the response body
* TCP: can configure timeout; can optionally send a payload and verify a newline-terminated response
* ICMP ping: can configure maximum allowed packet loss out of 5 packets
* SSL Expiration: can configure the number of days before the certificate expires, e.g. send an alert if the certificate is expired or expiring within 10 days
* DNS: can configure nameserver, record type, DNS name, and a string that should appear in the DNS response

**Monitoring.** Each check is configured with an `interval` and a `delay`, and there is a global `confirmations` parameter. The check action is performed every `interval` seconds. `confirmations` is how many workers need to agree before flipping the check state (from online to offline or offline to online), and `delay` is the number of intervals we need to see the new check state before flipping the state. For example, if a check is currently online with `interval=60`, `confirmations=4`, and `delay=3`, then the check is only marked offline if the check action repeatedly fails for 3 minutes, and 4 workers agree that it fails.

Contacts
--------

Contacts can be e-mail, call/SMS via Twilio, or webhook (i.e., perform an HTTP request).

Setup
-----

First, create a MySQL database and initialize tables with `install.sql`:

	$ mysql -u root
	> CREATE DATABASE gobearmon;
	> use gobearmon;
	> source install.sql;

Then, compile gobearmon:

	$ go build cmd/gobearmon.go

Configure gobearmon.worker.cfg gobearmon.viewserver.cfg, and copy the compiled gobearmon binary and corresponding configuration file to N workers and 1 viewserver. Run gobearmon:

	$ ./gobearmon gobearmon.worker.cfg
	or
	$ ./gobearmon gobearmon.viewserver.cfg

You can now configure the database. Configuration options are JSON-encoded, see check_params.go for valid options. Add a simple ping check:

	INSERT INTO checks (name, type, data) VALUES ('ping ipv6', 'icmp', '{"target":"example.com","force_ip":6}');

Here's a more complex HTTP check, which also sets the check interval/delay:

	INSERT INTO checks (name, type, data, check_interval, delay) VALUES ('my http check', 'http', '{"url":"https:\/\/example.com","method":"GET","expect_status":200,"timeout":15}', 120, 5);

Next, add a contact:

	INSERT INTO contacts (type, data) VALUES ('email', 'admin@example.com');

Finally, link the check and contact with an alert. For the check type, `offline` means the contact will only be notified when the check goes offline, `online` means only when it comes back online, and `both` means notified in both cases.

	INSERT INTO alerts (check_id, contact_id, type) VALUES (1, 1, 'both');
