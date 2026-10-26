package voice.backend.auth.linkoauth;

import java.nio.charset.StandardCharsets;
import java.util.*;
import voice.backend.auth.repository.LinkedIdentity;

/** Tagged envelopes preserve readable legacy rows while encrypting new transport sources. */
public final class LinkedOAuthSourceTokens {
  private static final byte[] PREFIX = {'V','L','O','1'};
  private final LinkedOAuthVault vault;
  public LinkedOAuthSourceTokens(byte[] key) { vault=new LinkedOAuthVault(key); }
  static byte[] seal(LinkedOAuthVault vault,UUID account,UUID profile,String provider,String purpose,String token) {
    byte[] encrypted=vault.seal(profile,"source-"+account+"-"+provider+"-"+purpose,token.getBytes(StandardCharsets.UTF_8));
    byte[] out=Arrays.copyOf(PREFIX,PREFIX.length+encrypted.length); System.arraycopy(encrypted,0,out,PREFIX.length,encrypted.length); return out;
  }
  public String open(LinkedIdentity row,byte[] token,String purpose) {
    if(token==null)return null;
    if(!tagged(token))return new String(token,StandardCharsets.UTF_8);
    return new String(vault.open(row.profileId(),"source-"+row.accountId()+"-"+row.platform()+"-"+purpose,
        Arrays.copyOfRange(token,PREFIX.length,token.length)),StandardCharsets.UTF_8);
  }
  public static boolean tagged(byte[] token) { return token!=null && token.length>=4 && Arrays.equals(PREFIX,Arrays.copyOf(token,4)); }
}
