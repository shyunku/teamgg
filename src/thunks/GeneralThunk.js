import axios from "axios";
import { authStore, getAuth } from "../stores/AuthStore";
import { toasts } from "svelte-toasts";

const normalizeServerHost = (host) => {
  const value = String(host ?? "")
    .trim()
    .replace(/\/+$/, "");
  if (!value) throw new Error("Server host is not configured");
  if (/^https?:\/\//i.test(value)) return value;

  const isLocalHost = /^(localhost|127\.0\.0\.1|\[::1\])(?::\d+)?$/i.test(value);
  return `${isLocalHost ? "http" : "https"}://${value}`;
};

export const ServerHostBase = normalizeServerHost(APP_SERVER_HOST);
const ServerHost = `${ServerHostBase}/v1`;
let dataDragonVersion = "";

export const initializeServerRuntime = async () => {
  const response = await axios.get(`${ServerHostBase}/`, { timeout: 3000 });
  dataDragonVersion = String(response.data?.dataDragonVersion ?? "").trim();
};

const versionedIconUrl = (url) => {
  if (!dataDragonVersion) return url;
  const separator = url.includes("?") ? "&" : "?";
  return `${url}${separator}v=${encodeURIComponent(dataDragonVersion)}`;
};

console.log("ServerHost", ServerHost);

const instance = axios.create({
  baseURL: ServerHost,
  withCredentials: false,
});

let refreshPromise = null;
let sessionExpiryHandled = false;

const refreshAuthToken = async () => {
  if (refreshPromise == null) {
    refreshPromise = (async () => {
      const refreshToken = getAuth()?.refreshToken;
      if (!refreshToken) throw new Error("Refresh token is missing");
      const response = await axios.post(`${ServerHost}/auth/refresh`, { refreshToken });
      const nextAuth = {
        ...getAuth(),
        authorized: true,
        accessToken: response.data.accessToken,
        refreshToken: response.data.refreshToken,
      };
      authStore.set(nextAuth);
      sessionExpiryHandled = false;
      return nextAuth.accessToken;
    })().finally(() => {
      refreshPromise = null;
    });
  }
  return refreshPromise;
};

const handleSessionExpiry = () => {
  if (sessionExpiryHandled) return;
  sessionExpiryHandled = true;
  authStore.initialize();
  toasts.warning("세션이 만료되었습니다. 다시 로그인해주세요.");
  window.location.href = "#/login";
};

instance.interceptors.request.use(
  (config) => {
    const { accessToken } = getAuth() ?? {};
    if (accessToken) {
      config.headers = config.headers ?? {};
      config.headers["Authorization"] = `Bearer ${accessToken}`;
    }
    return config;
  },
  (error) => {
    console.error(error);
    // 요청 에러 직전 호출됩니다.
    return Promise.reject(error);
  },
);

instance.interceptors.response.use(
  (resp) => {
    return resp;
  },
  async (error) => {
    const status = error?.response?.status;
    if (status === 401 && error?.config?.skipAuthRefresh !== true) {
      if (error.config._teamggAuthRetried === true) {
        handleSessionExpiry();
        return Promise.reject(error);
      }
      error.config._teamggAuthRetried = true;
      try {
        const accessToken = await refreshAuthToken();
        const cfg = error.config;
        cfg.headers = cfg.headers ?? {};
        cfg.headers["Authorization"] = `Bearer ${accessToken}`;
        return instance.request(cfg);
      } catch (_) {
        handleSessionExpiry();
      }
    }
    return Promise.reject(error);
  },
);

export const login = async (id, encryptedPassword) => {
  const response = await instance.post(
    `/auth/login`,
    {
      userId: id,
      encryptedPassword: encryptedPassword,
    },
    { skipAuthRefresh: true },
  );
  sessionExpiryHandled = false;
  return response.data;
};

export const signup = async (id, encryptedPassword) => {
  const response = await instance.post(
    `/auth/signup`,
    {
      userId: id,
      encryptedPassword: encryptedPassword,
    },
    { skipAuthRefresh: true },
  );
  return response.data;
};

export const startRsoLogin = async () => {
  const response = await instance.post(`/auth/rso/start`);
  return response.data;
};

export const startRsoLink = async () => {
  const response = await instance.post(`/auth/rso/link/start`);
  return response.data;
};

export const getRsoLoginStatus = async (flowId) => {
  const response = await instance.get(`/auth/rso/status`, {
    params: { flowId },
  });
  return response.data;
};

export const completeRsoWithExistingAccount = async (setupToken, userId, encryptedPassword) => {
  const response = await instance.post(
    `/auth/rso/complete/existing`,
    {
      setupToken,
      userId,
      encryptedPassword,
    },
    { skipAuthRefresh: true },
  );
  sessionExpiryHandled = false;
  return response.data;
};

export const completeRsoWithNewAccount = async (setupToken) => {
  const response = await instance.post(`/auth/rso/complete/new`, { setupToken });
  sessionExpiryHandled = false;
  return response.data;
};

export const getMyAccount = async () => {
  const response = await instance.get(`/auth/me`);
  return response.data;
};

export const unlinkRiotAccount = async (puuid = null) => {
  const response = await instance.delete(`/auth/me/riot`, {
    params: puuid ? { puuid } : undefined,
  });
  return response.data;
};

export const setPrimaryRiotAccount = async (puuid) => {
  const response = await instance.put(`/auth/me/riot/primary`, { puuid });
  return response.data;
};

export const logout = async () => {
  const response = await instance.post(`/auth/logout`);
  return response.data;
};

export const testTokenReq = async () => {
  const response = await instance.get(`/platform/tokenTest`);
  return response.data;
};

export const getSummonerInfoReq = async (gameName = null, tagLine = null) => {
  const encodedGameName = encodeURIComponent(gameName);
  const encodedTagLine = encodeURIComponent(tagLine);
  const response = await instance.get(`/summoner?gameName=${encodedGameName}&tagLine=${encodedTagLine}`);
  return response.data;
};

export const getSummonerInfoByPuuidReq = async (puuid) => {
  const response = await instance.get(`/summoner-by-puuid?puuid=${puuid}`);
  return response.data;
};

export const quickSearchSummonerReq = async (keyword) => {
  const response = await instance.get(`/quickSearch?keyword=${encodeURIComponent(keyword ?? "")}`);
  return response.data;
};

export const renewSummonerInfoReq = async (puuid) => {
  const response = await instance.post(`/renewSummoner`, {
    puuid: puuid,
  });
  return response.data;
};

export const getMatchesReq = async (puuid, queueId) => {
  const response = await instance.get(`/matches?puuid=${puuid}&queueId=${queueId}`);
  return response.data;
};

export const loadMoreMatches = async (puuid, queueId, before) => {
  const response = await instance.post(`/loadMatches`, {
    puuid: puuid,
    queueId: queueId,
    before: before,
  });
  return response.data;
};

export const getIngameInfo = async (puuid) => {
  const response = await instance.get(`/ingame?puuid=${puuid}`);
  return response.data;
};

/* ---------------------- platform ---------------------- */
export const getCustomGameConfigurations = async () => {
  const response = await instance.get(`/platform/custom-game/list`);
  return response.data;
};

export const getJoinedCustomGameConfigurations = async () => {
  const response = await instance.get(`/platform/custom-game/joined-list`);
  return response.data;
};

export const createCustomGameConfiguration = async (config) => {
  const response = await instance.post(`/platform/custom-game/create`);
  return response.data;
};

export const getCustomGameConfigurationInfo = async (id) => {
  const response = await instance.get(`/platform/custom-game/info?id=${id}`);
  return response.data;
};

export const updateCustomGameConfigurationName = async (id, name) => {
  const response = await instance.patch(`/platform/custom-game/name`, { id, name });
  return response.data;
};

export const addCustomGameCandidateReq = async (customGameConfigId, name, tagLine) => {
  const response = await instance.put(`/platform/custom-game/candidate`, {
    customGameConfigId,
    name,
    tagLine,
  });

  return response.data;
};

export const deleteCustomGameCandidateReq = async (customGameConfigId, puuid) => {
  const response = await instance.delete(
    `/platform/custom-game/candidate?customGameConfigId=${customGameConfigId}&puuid=${puuid}`,
  );

  return response.data;
};

export const arrangeCustomGameParticipantReq = async (customGameConfigId, puuid, team, position) => {
  const response = await instance.post(`/platform/custom-game/arrange`, {
    customGameConfigId,
    puuid,
    team,
    targetPosition: position,
  });

  return response.data;
};

export const unArrangeCustomGameParticipantReq = async (customGameConfigId, puuid) => {
  const response = await instance.post(`/platform/custom-game/unarrange`, {
    customGameConfigId,
    puuid,
  });

  return response.data;
};

export const setCustomGameCandidateFavorPositionReq = async (customGameConfigId, puuid, position, strength) => {
  const response = await instance.post(`/platform/custom-game/favor-position`, {
    customGameConfigId,
    puuid,
    favorPosition: position,
    strength,
  });

  return response.data;
};

export const setCustomGameCandidateLineMasteryReq = async (customGameConfigId, puuid, position, level) => {
  const response = await instance.post(`/platform/custom-game/line-mastery`, {
    customGameConfigId,
    puuid,
    position,
    level,
  });
  return response.data;
};

export const saveCustomGameDefaultFavorPositionReq = async (customGameConfigId, puuid) => {
  const response = await instance.post(`/platform/custom-game/default-favor-position`, {
    customGameConfigId,
    puuid,
  });
  return response.data;
};

export const resetCustomGameFavorPositionReq = async (customGameConfigId, puuid) => {
  const response = await instance.post(`/platform/custom-game/reset-favor-position`, {
    customGameConfigId,
    puuid,
  });
  return response.data;
};

export const setCustomGameCandidateCustomTierRankReq = async (customGameConfigId, puuid, tier, rank) => {
  const response = await instance.post(`/platform/custom-game/custom-tier-rank`, {
    customGameConfigId,
    puuid,
    tier,
    rank,
  });

  return response.data;
};

export const deleteCustomGameParticipantColorCodeReq = async (customGameConfigId) => {
  const response = await instance.delete(
    `/platform/custom-game/custom-color-label?customGameConfigId=${customGameConfigId}`,
  );
  return response.data;
};

export const setCustomGameParticipantColorCodeReq = async (customGameConfigId, puuid, colorCode) => {
  const response = await instance.post(`/platform/custom-game/custom-color-label`, {
    customGameConfigId,
    puuid,
    colorCode,
  });

  return response.data;
};

export const getCustomGameBalanceReq = async (customGameConfigId) => {
  const response = await instance.get(`/platform/custom-game/balance?id=${customGameConfigId}`);
  return response.data;
};

export const findMostBalancedCustomGameReq = async (customGameConfigId, weights) => {
  const response = await instance.post(`/platform/custom-game/optimize`, {
    id: customGameConfigId,
    ...weights,
  });
  return response.data;
};

export const arrangeAllCandidatesReq = async (customGameConfigId) => {
  const response = await instance.post(`/platform/custom-game/arrange-all`, {
    id: customGameConfigId,
  });
  return response.data;
};

export const unArrangeAllCandidatesReq = async (customGameConfigId) => {
  const response = await instance.post(`/platform/custom-game/unarrange-all`, {
    id: customGameConfigId,
  });
  return response.data;
};

export const swapCustomGameTeamReq = async (customGameConfigId, puuid) => {
  const response = await instance.post(`/platform/custom-game/swap-team`, {
    id: customGameConfigId,
    puuid,
  });
  return response.data;
};

export const shuffleCustomGameTeamReq = async (customGameConfigId) => {
  const response = await instance.post(`/platform/custom-game/shuffle`, {
    id: customGameConfigId,
  });
  return response.data;
};

export const renewCustomGameTeamRankReq = async (customGameConfigId) => {
  const response = await instance.post(`/platform/custom-game/renew-ranks`, {
    id: customGameConfigId,
  });
  return response.data;
};

export const getTierRankByRatingPointReq = async (ratingPoint) => {
  const response = await instance.get(`/platform/custom-game/tier-rank?ratingPoint=${ratingPoint}`);
  return response.data;
};

/* ---------------------- statistics ---------------------- */
const StatisticsClientCacheTtl = 5 * 60 * 1000;
const statisticsResponseCache = new Map();

const getStatisticsCached = async (key, path) => {
  const now = Date.now();
  const cached = statisticsResponseCache.get(key);
  if (cached?.data != null && cached.expiresAt > now) return cached.data;
  if (cached?.promise != null) return cached.promise;

  const promise = instance
    .get(path)
    .then((response) => {
      statisticsResponseCache.set(key, {
        data: response.data,
        expiresAt: Date.now() + StatisticsClientCacheTtl,
        promise: null,
      });
      return response.data;
    })
    .catch((error) => {
      statisticsResponseCache.delete(key);
      throw error;
    });
  statisticsResponseCache.set(key, { data: cached?.data ?? null, expiresAt: 0, promise });
  return promise;
};

export const getChampionStatisticsReq = async () => {
  return getStatisticsCached("champion", `/platform/statistics/champion`);
};

export const getChampionDetailStatisticsReq = async (championId) => {
  return getStatisticsCached(
    `champion-detail:${championId}`,
    `/platform/statistics/champion-detail?championId=${championId}`,
  );
};

let metaStatisticsPath = `/platform/statistics/meta-summary`;
export const getMetaStatisticsReq = async () => {
  try {
    return await getStatisticsCached("meta", metaStatisticsPath);
  } catch (error) {
    if (error?.response?.status !== 404 || metaStatisticsPath.endsWith("/meta")) throw error;
    metaStatisticsPath = `/platform/statistics/meta`;
    return getStatisticsCached("meta", metaStatisticsPath);
  }
};

export const getFullMetaStatisticsReq = async () => {
  return getStatisticsCached("meta-full", `/platform/statistics/meta`);
};

export const getTierStatisticsReq = async () => {
  return getStatisticsCached("tier", `/platform/statistics/tier`);
};

export const getMasteryStatisticsReq = async () => {
  return getStatisticsCached("mastery", `/platform/statistics/mastery`);
};

/* ---------------------- links ---------------------- */

export const profileIconUrl = (profileIconId = 0) => {
  // if (profileIconId == 0) return null;
  return versionedIconUrl(`${ServerHost}/icon/profile?id=${profileIconId}`);
};

export const championIconUrl = (championId = 0) => {
  if (championId == 0) return null;
  return versionedIconUrl(`${ServerHost}/icon/champion?key=${championId}`);
};

export const centeredChampionSplashUrl = (championId = 0) => {
  if (championId == 0) return null;
  return versionedIconUrl(`${ServerHost}/icon/centered-splash-champion?key=${championId}`);
};

export const summonerSpellIconUrl = (spellId = 0) => {
  if (spellId == 0) return null;
  return versionedIconUrl(`${ServerHost}/icon/summonerSpell?id=${spellId}`);
};

export const itemIconUrl = (itemId = 0) => {
  if (itemId == 0) return null;
  return versionedIconUrl(`${ServerHost}/icon/item?id=${itemId}`);
};

export const perkStyleIconUrl = (perkStyleId = 0) => {
  if (perkStyleId == 0) return null;
  perkStyleId = parseInt(perkStyleId);
  return versionedIconUrl(`${ServerHost}/icon/perkStyle?id=${perkStyleId}`);
};
