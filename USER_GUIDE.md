# Shclop user guide

This guide describes the current user flow for creating and using OpenClaw/NanoClaw agents in a self-hosted Shclop installation.

## Sign in

Shclop uses local accounts. An administrator creates your account and gives you a username and password.

Open the Shclop URL provided by your operator, sign in, and use **Log out** from the user menu when finished.

If your account is disabled, login fails. Contact an administrator.

## Create an agent

Agents are owned by the user who creates them.

1. Open **Agents**.
2. Select **Create agent**.
3. Enter an agent name.
4. Choose a runtime:
   - **OpenClaw** for the OpenClaw runtime image;
   - **NanoClaw** for the NanoClaw runtime image.
5. Select an enabled model from the model list.
6. Save the agent.

Only administrator-enabled models are available. If a model is later disabled, the agent cannot be started with that model until an administrator enables it again or you choose another enabled model.

## Start and stop an agent

To start an agent:

1. Open **Agents**.
2. Select your agent.
3. Choose **Start**.

In production, starting an agent asks the backend to create a Kubernetes runtime pod using the configured Kata RuntimeClass. The pod receives the selected model, the LLM gateway base URL, and an API key SecretKeyRef. Shclop does not expose the raw API key in the UI.

To stop an agent:

1. Open the agent.
2. Choose **Stop**.

Stopping an agent asks the runtime provider to stop the Kubernetes resources for that agent.

## Chat with an agent

After the agent is running:

1. Open the agent chat.
2. Type a task or question.
3. Send the message.
4. Watch streamed responses and status events.

Runtime work is performed by the selected OpenClaw/NanoClaw runtime image. The runtime connects back to Shclop through the runtime WebSocket and streams events to the browser chat.

## Activity

The activity view shows user-facing events such as:

- login;
- agent creation;
- start request;
- runtime start result;
- chat task routing;
- stop request.

Regular users see their own activity. Admins can see broader platform activity where exposed by the admin UI.

## Current limitations

The current user path does not include:

- workspaces;
- skills or catalogs;
- MCP tools;
- third-party integrations;
- user-managed provider credentials;
- approval workflows for security policies.

If an expected model, runtime, or gateway is unavailable, contact an administrator.
