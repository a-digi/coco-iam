import { postPublicApi} from './client';
import type { AuthToken, AuthResponse } from '../Components/Auth/Guard/model/auth';

export async function renewToken(refresh_token: string): Promise<AuthToken> {
  const response = await postPublicApi('admin/oauth/renew', { refresh_token: refresh_token }) as AuthResponse;

  if (response && response.message && response.message.access_token) {
    localStorage.setItem('auth_token', JSON.stringify(response.message));

    return response.message;
  }

  console.error(response);
  throw new Error('Invalid token renew response');
}
