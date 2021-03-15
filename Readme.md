<div align="center">
	<h1><img alt="Hermes logo" src="logo.png" height="300" /><br />
	</h1>
</div>



[comment]: <> (![Hermes logo]&#40;logo.png "Hermes"&#41;)


# Hermes


The multifuntional event delivery service

## Glossary


- Session - representation of a single WebSocket connection. A session has two states:
    - Anonymous - by default, all connections are anonymous and can only receive `broadcast` messages;
    - Authorized - a session can be authorized by the `authorization` command. An authorized session contains information about the user and can receive `direct` messages.
 
 - User - one user can have several *Sessions*

## Communication protocol

### Event Publisher  

> Hermes currently only supports RabbitMQ as an event provider.

#### RabbitMQ  

Some services may publish the event on the RabbitMQ exchange, and Hermes will deliver it to Users.
Hermes does not process AMQP message body, only message headers and meta.

Headers list:

| Header     | Values                            | Required | Description |
| ---------- | --------------------------------- | -------- | ----------- |
| uuid       | any string                        | false    | User unique identifier. It is needed to delivery `direct`  messages |
| visibility | `broadcast`, `direct`, `internal` | true     | `broadcast` - sent to all connected sessions, `direct` - sent directly to a user for all of his sessions, `internal` - ignored |
| event_type | any string                        | true     | Identifier of the event kind |
| role       | `any`; any string                 | false    | By default, all messages will be sent to any user according to `visibility`. If this header is present, this message will be sent only if the role of the authorized user matches |
| cache_ttl  | `-1`, `0`; int, number of seconds | false    |             |


### Client

##### Establish WS connection using this url, where:

- `<server_address>` — `ip:port`, `host:port` or `my.some.domain`

```
ws://<server_address>/_ws/subscribe
```

> One unique user can establish many sessions from different clients. The service will deliver a `direct` event to each of them.
    
##### Send handshake

```json
{
  "channel":"ws_status",
  "event": "handshake"
}
```

##### Subscribe to event channel(s):

```json
{
  "channel":"ws_status",
  "event": "subscribe",
  "command": {
    "channel":"ctp.notifications.prices",
    "event": "market_data"
  }
}
```

Instead channel name can be passed **`***`**. 
**`***`** is a reserved symbol for wildcard subscription.

Field `command.event` is optional and can be omitted.

##### Authorize connection

To receive direct event bounded to some `user` client must preform authorization

— `<your secret token>` - any string used as a token to identify a user session, the service does not process it. This token will be used to request authentication as is.
- `<client origin>` - some identifier of the client source application, for example: `user`, `admin`. Depends on configuration. The value is used to select **AuthProvider** for this client. 

Client message: 

```json
{
  "channel":"ws_status",
  "event": "authorize",
  "command": {
    "token": "<your secret token>",
    "origin": "<client origin>",
    "role": "<client role>"
  }
}
```
 
Server Response:

```json
{
  "channel":"ws_status",
  "event": "authorize",
  "data": {
    "resultCode": "code",
    "authorize": "status"
  }
}
```

- `status` - string; status of authorization: `success` or `failed`
- `code` - number; status code of authorization, matches with HTTP Status Codes

    - 200 — All fine, user authorized

    - 401 — Hermes access key invalid, Hermes can't access to this resource;

    - 403 — Client token invalid, Client can't be authorized;
    
    - 406 - Invalid **origin** or **role**; 

    - Other codes will be interpreted as Error code and Client will be not authorized;

##### Server Ping

Server sends ping to all clients with some period:

```json
{
  "channel": "ws_status", 
  "event": "ping"
}
```

Client must send response:

```json
{
  "channel": "ws_status", 
  "event": "pong"
}
```

Otherwise connection will be dropped.

### Events

Event has next format:

```json
{
  "broadcast": true,
  "channel": "<channel>",
  "event": "<event>",
  "data": {
    "<event>": {}
  }
}
```

- `broadcast` - indicates that this is a broadcast event, otherwise it is direct to the user.
- `data` - this field contains a field whose name is equal to the value of the `event` field. 

Example:

```json
{
  "broadcast": true,
  "channel": "ctp.notifications.prices",
  "event": "market_data",
  "data": {
    "market_data": {
      "symbol": "BCHBTC",
      "price": 0.03828405,
      "volume": 42809.069,
      "source": "coingecko",
      "time": "2020-01-20T08:28:40.68014Z"
    }
  }
}
``` 

## Authorization


Accepted Status Codes from auth provider:

- 200 — All fine

- 401 — Hermes access key invalid, Hermes can't access to this resource;

- 403 — Client token invalid, Client can't be authorized

- Other codes will be interpreted as Error code and Client will be not authorized


## Credits

Logo: https://openclipart.org/detail/171035/winged-foot
