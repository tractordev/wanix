
exports.onExecutePostLogin = async (event, api) => {
  if (event.user.nickname && event.user.nickname !== event.secrets.admin) {
    api.access.deny(`Access to ${event.client.name} is not allowed for ${event.user.nickname}`);
  } else {
    const AuthenticationClient = require("auth0").AuthenticationClient;
    const auth = new AuthenticationClient({
      domain: event.secrets.domain,
      clientId: event.secrets.clientId,
      clientSecret: event.secrets.clientSecret
    });
    const grant = await auth.oauth.clientCredentialsGrant({ audience: `https://${event.secrets.domain}/api/v2/` });
    const ManagementClient = require('auth0').ManagementClient;
    const management = new ManagementClient({
        domain: event.secrets.domain,
        token: grant.data.access_token,
    });
    const resp = await management.users.get({id: event.user.user_id});
    api.user.setUserMetadata("gh_token", resp.data.identities[0].access_token);
    
  }
};