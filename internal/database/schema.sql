--
-- PostgreSQL database dump
--

CREATE SCHEMA IF NOT EXISTS suwayomi;


--
-- Name: delete_manga_category_update_version(); Type: FUNCTION; Schema: suwayomi; Owner: -
--

CREATE FUNCTION suwayomi.delete_manga_category_update_version() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    UPDATE suwayomi.mangauser SET version = version + 1
    WHERE manga = OLD.manga
      AND user_id = OLD.user_id
      AND is_syncing = FALSE;

    RETURN OLD;
END;
$$;


--
-- Name: insert_category_uid(); Type: FUNCTION; Schema: suwayomi; Owner: -
--

CREATE FUNCTION suwayomi.insert_category_uid() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NEW.uid = 0 THEN
        NEW.uid := (floor(random() * 9223372036854775806) + 1)::bigint;
    END IF;

    IF NEW.last_modified_at = 0 THEN
        NEW.last_modified_at := EXTRACT(EPOCH FROM NOW());
    END IF;

    RETURN NEW;
END;
$$;


--
-- Name: insert_manga_category_update_version(); Type: FUNCTION; Schema: suwayomi; Owner: -
--

CREATE FUNCTION suwayomi.insert_manga_category_update_version() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    UPDATE suwayomi.mangauser SET version = version + 1
    WHERE manga = NEW.manga
      AND user_id = NEW.user_id
      AND is_syncing = FALSE;

    RETURN NEW;
END;
$$;


--
-- Name: trackrecord_update_manga_version(); Type: FUNCTION; Schema: suwayomi; Owner: -
--

CREATE FUNCTION suwayomi.trackrecord_update_manga_version() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    UPDATE suwayomi.mangauser SET version = version + 1
    WHERE manga = COALESCE(NEW.manga_id, OLD.manga_id)
      AND user_id = COALESCE(NEW.user_id, OLD.user_id)
      AND is_syncing = FALSE;

    RETURN COALESCE(NEW, OLD);
END;
$$;


--
-- Name: update_category_version(); Type: FUNCTION; Schema: suwayomi; Owner: -
--

CREATE FUNCTION suwayomi.update_category_version() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NOT NEW.is_syncing
       AND ROW(NEW.name, NEW.sort_order)
           IS DISTINCT FROM
           ROW(OLD.name, OLD.sort_order)
    THEN
        NEW.version := NEW.version + 1;
        NEW.last_modified_at := EXTRACT(EPOCH FROM NOW());
    END IF;

    RETURN NEW;
END;
$$;


--
-- Name: update_chapter_user_last_modified_at(); Type: FUNCTION; Schema: suwayomi; Owner: -
--

CREATE FUNCTION suwayomi.update_chapter_user_last_modified_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NEW.is_syncing THEN
        RETURN NEW;
    END IF;

    IF TG_OP = 'UPDATE'
       AND ROW(NEW.read, NEW.bookmark, NEW.last_page_read, NEW.version)
           IS NOT DISTINCT FROM
           ROW(OLD.read, OLD.bookmark, OLD.last_page_read, OLD.version)
    THEN
        RETURN NEW;
    END IF;

    NEW.last_modified_at := EXTRACT(EPOCH FROM NOW());
    RETURN NEW;
END;
$$;


--
-- Name: update_chapter_user_version(); Type: FUNCTION; Schema: suwayomi; Owner: -
--

CREATE FUNCTION suwayomi.update_chapter_user_version() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NOT NEW.is_syncing
       AND ROW(NEW.read, NEW.bookmark, NEW.last_page_read)
           IS DISTINCT FROM
           ROW(OLD.read, OLD.bookmark, OLD.last_page_read)
    THEN
        NEW.version := NEW.version + 1;
    END IF;

    RETURN NEW;
END;
$$;


--
-- Name: update_manga_bump_user_versions(); Type: FUNCTION; Schema: suwayomi; Owner: -
--

CREATE FUNCTION suwayomi.update_manga_bump_user_versions() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF ROW(NEW.url, NEW.description) IS DISTINCT FROM ROW(OLD.url, OLD.description)
    THEN
        UPDATE suwayomi.mangauser
        SET version = version + 1,
            last_modified_at = EXTRACT(EPOCH FROM NOW())
        WHERE manga = NEW.id
          AND is_syncing = FALSE;
    END IF;

    RETURN NEW;
END;
$$;


--
-- Name: update_manga_user_last_modified_at(); Type: FUNCTION; Schema: suwayomi; Owner: -
--

CREATE FUNCTION suwayomi.update_manga_user_last_modified_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NEW.is_syncing THEN
        RETURN NEW;
    END IF;

    IF TG_OP = 'UPDATE'
       AND ROW(NEW.in_library, NEW.in_library_at, NEW.version)
           IS NOT DISTINCT FROM
           ROW(OLD.in_library, OLD.in_library_at, OLD.version)
    THEN
        RETURN NEW;
    END IF;

    NEW.last_modified_at := EXTRACT(EPOCH FROM NOW());
    RETURN NEW;
END;
$$;


--
-- Name: update_manga_user_version(); Type: FUNCTION; Schema: suwayomi; Owner: -
--

CREATE FUNCTION suwayomi.update_manga_user_version() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NOT NEW.is_syncing
       AND ROW(NEW.in_library, NEW.in_library_at)
           IS DISTINCT FROM
           ROW(OLD.in_library, OLD.in_library_at)
    THEN
        NEW.version := NEW.version + 1;
    END IF;

    RETURN NEW;
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: category; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.category (
    id integer NOT NULL,
    name character varying(64) NOT NULL,
    is_default boolean DEFAULT false NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    include_in_update integer DEFAULT '-1'::integer,
    include_in_download integer DEFAULT '-1'::integer,
    version bigint DEFAULT 0 NOT NULL,
    uid bigint DEFAULT 0 NOT NULL,
    is_syncing boolean DEFAULT false NOT NULL,
    last_modified_at bigint DEFAULT 0 NOT NULL,
    flags integer DEFAULT 0 NOT NULL,
    user_id integer NOT NULL,
    is_default_category boolean DEFAULT false NOT NULL,
    default_user_id integer GENERATED ALWAYS AS (
CASE
    WHEN is_default_category THEN user_id
    ELSE NULL::integer
END) STORED
);


--
-- Name: category_id_seq; Type: SEQUENCE; Schema: suwayomi; Owner: -
--

CREATE SEQUENCE suwayomi.category_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: category_id_seq; Type: SEQUENCE OWNED BY; Schema: suwayomi; Owner: -
--

ALTER SEQUENCE suwayomi.category_id_seq OWNED BY suwayomi.category.id;


--
-- Name: categorymanga; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.categorymanga (
    id integer NOT NULL,
    category integer NOT NULL,
    manga integer NOT NULL,
    user_id integer NOT NULL
);


--
-- Name: categorymanga_id_seq; Type: SEQUENCE; Schema: suwayomi; Owner: -
--

CREATE SEQUENCE suwayomi.categorymanga_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: categorymanga_id_seq; Type: SEQUENCE OWNED BY; Schema: suwayomi; Owner: -
--

ALTER SEQUENCE suwayomi.categorymanga_id_seq OWNED BY suwayomi.categorymanga.id;


--
-- Name: categorymeta; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.categorymeta (
    id integer NOT NULL,
    meta_key character varying(256) NOT NULL,
    value character varying(4096) NOT NULL,
    category_ref integer DEFAULT 0,
    user_id integer NOT NULL
);


--
-- Name: categorymeta_id_seq; Type: SEQUENCE; Schema: suwayomi; Owner: -
--

CREATE SEQUENCE suwayomi.categorymeta_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: categorymeta_id_seq; Type: SEQUENCE OWNED BY; Schema: suwayomi; Owner: -
--

ALTER SEQUENCE suwayomi.categorymeta_id_seq OWNED BY suwayomi.categorymeta.id;


--
-- Name: chapter; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.chapter (
    id integer NOT NULL,
    url character varying(2048) NOT NULL,
    name character varying(512) NOT NULL,
    date_upload bigint DEFAULT 0 NOT NULL,
    chapter_number real DEFAULT '-1.0'::numeric NOT NULL,
    scanlator character varying(256),
    source_order integer NOT NULL,
    manga integer NOT NULL,
    is_downloaded boolean DEFAULT false,
    page_count integer DEFAULT '-1'::integer,
    fetched_at bigint DEFAULT 0,
    real_url character varying(2048) DEFAULT NULL::character varying,
    memo text DEFAULT '{}'::text NOT NULL
);


--
-- Name: chapter_id_seq; Type: SEQUENCE; Schema: suwayomi; Owner: -
--

CREATE SEQUENCE suwayomi.chapter_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chapter_id_seq; Type: SEQUENCE OWNED BY; Schema: suwayomi; Owner: -
--

ALTER SEQUENCE suwayomi.chapter_id_seq OWNED BY suwayomi.chapter.id;


--
-- Name: chaptermeta; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.chaptermeta (
    id integer NOT NULL,
    meta_key character varying(256) NOT NULL,
    value character varying(4096) NOT NULL,
    chapter_ref integer NOT NULL,
    user_id integer NOT NULL
);


--
-- Name: chaptermeta_id_seq; Type: SEQUENCE; Schema: suwayomi; Owner: -
--

CREATE SEQUENCE suwayomi.chaptermeta_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chaptermeta_id_seq; Type: SEQUENCE OWNED BY; Schema: suwayomi; Owner: -
--

ALTER SEQUENCE suwayomi.chaptermeta_id_seq OWNED BY suwayomi.chaptermeta.id;


--
-- Name: chapteruser; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.chapteruser (
    id integer NOT NULL,
    chapter integer NOT NULL,
    user_id integer NOT NULL,
    read boolean DEFAULT false NOT NULL,
    bookmark boolean DEFAULT false NOT NULL,
    last_page_read integer DEFAULT 0 NOT NULL,
    last_read_at bigint DEFAULT 0 NOT NULL,
    is_downloaded boolean DEFAULT false NOT NULL,
    is_download_requested boolean DEFAULT false NOT NULL,
    koreader_hash character varying(32),
    version bigint DEFAULT 0 NOT NULL,
    is_syncing boolean DEFAULT false NOT NULL,
    last_modified_at bigint DEFAULT 0 NOT NULL
);


--
-- Name: chapteruser_id_seq; Type: SEQUENCE; Schema: suwayomi; Owner: -
--

CREATE SEQUENCE suwayomi.chapteruser_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: chapteruser_id_seq; Type: SEQUENCE OWNED BY; Schema: suwayomi; Owner: -
--

ALTER SEQUENCE suwayomi.chapteruser_id_seq OWNED BY suwayomi.chapteruser.id;


--
-- Name: extension; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.extension (
    id integer NOT NULL,
    apk_name character varying(1024),
    icon_url character varying(2048) DEFAULT 'https://raw.githubusercontent.com/tachiyomiorg/tachiyomi/64ba127e7d43b1d7e6d58a6f5c9b2bd5fe0543f7/app/src/main/res/mipmap-xxxhdpi/ic_local_source.webp'::character varying NOT NULL,
    name character varying(128) NOT NULL,
    pkg_name character varying(128) NOT NULL,
    version_name character varying(16) NOT NULL,
    version_code bigint NOT NULL,
    lang character varying(32) NOT NULL,
    is_installed boolean DEFAULT false NOT NULL,
    has_update boolean DEFAULT false NOT NULL,
    is_obsolete boolean DEFAULT false NOT NULL,
    class_name character varying(1024) DEFAULT ''::character varying NOT NULL,
    store_index_url character varying(2048) DEFAULT NULL::character varying,
    extension_lib character varying(16) NOT NULL,
    apk_url character varying(2048),
    content_warning integer DEFAULT 0,
    jar_url character varying(2048) DEFAULT NULL::character varying
);


--
-- Name: extension_id_seq; Type: SEQUENCE; Schema: suwayomi; Owner: -
--

CREATE SEQUENCE suwayomi.extension_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: extension_id_seq; Type: SEQUENCE OWNED BY; Schema: suwayomi; Owner: -
--

ALTER SEQUENCE suwayomi.extension_id_seq OWNED BY suwayomi.extension.id;


--
-- Name: extensionstore; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.extensionstore (
    id integer NOT NULL,
    index_url character varying(2048) NOT NULL,
    name character varying(256) NOT NULL,
    badge_label character varying(32) NOT NULL,
    signing_key character varying(512) NOT NULL,
    contact_website character varying(2048) NOT NULL,
    contact_discord character varying(2048),
    is_legacy boolean DEFAULT false NOT NULL,
    extension_list_url character varying(2048)
);


--
-- Name: extensionstore_id_seq; Type: SEQUENCE; Schema: suwayomi; Owner: -
--

CREATE SEQUENCE suwayomi.extensionstore_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: extensionstore_id_seq; Type: SEQUENCE OWNED BY; Schema: suwayomi; Owner: -
--

ALTER SEQUENCE suwayomi.extensionstore_id_seq OWNED BY suwayomi.extensionstore.id;


--
-- Name: globalmeta; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.globalmeta (
    id integer NOT NULL,
    meta_key character varying(256) NOT NULL,
    value character varying(4096) NOT NULL,
    user_id integer NOT NULL
);


--
-- Name: globalmeta_id_seq; Type: SEQUENCE; Schema: suwayomi; Owner: -
--

CREATE SEQUENCE suwayomi.globalmeta_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: globalmeta_id_seq; Type: SEQUENCE OWNED BY; Schema: suwayomi; Owner: -
--

ALTER SEQUENCE suwayomi.globalmeta_id_seq OWNED BY suwayomi.globalmeta.id;


--
-- Name: manga; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.manga (
    id integer NOT NULL,
    url character varying(2048) NOT NULL,
    title character varying(512) NOT NULL,
    initialized boolean DEFAULT false NOT NULL,
    artist text,
    author text,
    description text,
    genre text,
    status integer DEFAULT 0 NOT NULL,
    thumbnail_url character varying(2048),
    source bigint NOT NULL,
    real_url character varying(2048) DEFAULT NULL::character varying,
    last_fetched_at bigint DEFAULT 0,
    chapters_last_fetched_at bigint DEFAULT 0,
    thumbnail_url_last_fetched bigint DEFAULT 0,
    update_strategy character varying(256) DEFAULT 'ALWAYS_UPDATE'::character varying,
    memo text DEFAULT '{}'::text NOT NULL
);


--
-- Name: manga_id_seq; Type: SEQUENCE; Schema: suwayomi; Owner: -
--

CREATE SEQUENCE suwayomi.manga_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: manga_id_seq; Type: SEQUENCE OWNED BY; Schema: suwayomi; Owner: -
--

ALTER SEQUENCE suwayomi.manga_id_seq OWNED BY suwayomi.manga.id;


--
-- Name: mangameta; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.mangameta (
    id integer NOT NULL,
    meta_key character varying(256) NOT NULL,
    value character varying(4096) NOT NULL,
    manga_ref integer NOT NULL,
    user_id integer NOT NULL
);


--
-- Name: mangameta_id_seq; Type: SEQUENCE; Schema: suwayomi; Owner: -
--

CREATE SEQUENCE suwayomi.mangameta_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mangameta_id_seq; Type: SEQUENCE OWNED BY; Schema: suwayomi; Owner: -
--

ALTER SEQUENCE suwayomi.mangameta_id_seq OWNED BY suwayomi.mangameta.id;


--
-- Name: mangauser; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.mangauser (
    id integer NOT NULL,
    manga integer NOT NULL,
    user_id integer NOT NULL,
    in_library boolean DEFAULT false NOT NULL,
    in_library_at bigint DEFAULT 0 NOT NULL,
    viewer integer DEFAULT 0 NOT NULL,
    viewer_flags integer,
    chapter_flags integer DEFAULT 0 NOT NULL,
    version bigint DEFAULT 0 NOT NULL,
    is_syncing boolean DEFAULT false NOT NULL,
    last_modified_at bigint DEFAULT 0 NOT NULL
);


--
-- Name: mangauser_id_seq; Type: SEQUENCE; Schema: suwayomi; Owner: -
--

CREATE SEQUENCE suwayomi.mangauser_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: mangauser_id_seq; Type: SEQUENCE OWNED BY; Schema: suwayomi; Owner: -
--

ALTER SEQUENCE suwayomi.mangauser_id_seq OWNED BY suwayomi.mangauser.id;


--
-- Name: migrations; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.migrations (
    version integer NOT NULL,
    name character varying(400) NOT NULL,
    executed_at timestamp without time zone NOT NULL
);


--
-- Name: page; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.page (
    id integer NOT NULL,
    index integer NOT NULL,
    url character varying(2048) NOT NULL,
    image_url text,
    chapter integer NOT NULL
);


--
-- Name: page_id_seq; Type: SEQUENCE; Schema: suwayomi; Owner: -
--

CREATE SEQUENCE suwayomi.page_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: page_id_seq; Type: SEQUENCE OWNED BY; Schema: suwayomi; Owner: -
--

ALTER SEQUENCE suwayomi.page_id_seq OWNED BY suwayomi.page.id;


--
-- Name: source; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.source (
    id bigint NOT NULL,
    name character varying(128) NOT NULL,
    lang character varying(32) NOT NULL,
    extension integer NOT NULL,
    content_warning integer DEFAULT 0
);


--
-- Name: sourcemeta; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.sourcemeta (
    id integer NOT NULL,
    meta_key character varying(256) NOT NULL,
    value character varying(4096) NOT NULL,
    source_ref bigint NOT NULL,
    user_id integer NOT NULL
);


--
-- Name: sourcemeta_id_seq; Type: SEQUENCE; Schema: suwayomi; Owner: -
--

CREATE SEQUENCE suwayomi.sourcemeta_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: sourcemeta_id_seq; Type: SEQUENCE OWNED BY; Schema: suwayomi; Owner: -
--

ALTER SEQUENCE suwayomi.sourcemeta_id_seq OWNED BY suwayomi.sourcemeta.id;


--
-- Name: trackrecord; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.trackrecord (
    id integer NOT NULL,
    manga_id integer NOT NULL,
    sync_id integer NOT NULL,
    remote_id bigint NOT NULL,
    library_id bigint,
    title character varying(512) NOT NULL,
    last_chapter_read double precision NOT NULL,
    total_chapters integer NOT NULL,
    status integer NOT NULL,
    score double precision NOT NULL,
    remote_url character varying(512) NOT NULL,
    start_date bigint NOT NULL,
    finish_date bigint NOT NULL,
    private boolean DEFAULT false NOT NULL,
    user_id integer NOT NULL
);


--
-- Name: trackrecord_id_seq; Type: SEQUENCE; Schema: suwayomi; Owner: -
--

CREATE SEQUENCE suwayomi.trackrecord_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: trackrecord_id_seq; Type: SEQUENCE OWNED BY; Schema: suwayomi; Owner: -
--

ALTER SEQUENCE suwayomi.trackrecord_id_seq OWNED BY suwayomi.trackrecord.id;


--
-- Name: tracksearch; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.tracksearch (
    id integer NOT NULL,
    tracker_id integer NOT NULL,
    remote_id bigint NOT NULL,
    title character varying(512) NOT NULL,
    total_chapters integer NOT NULL,
    tracking_url character varying(512) NOT NULL,
    cover_url character varying(512) NOT NULL,
    summary character varying(4096) NOT NULL,
    publishing_status character varying(512) NOT NULL,
    publishing_type character varying(512) NOT NULL,
    start_date character varying(128) NOT NULL,
    library_id bigint,
    last_chapter_read double precision DEFAULT 0 NOT NULL,
    status integer DEFAULT 0 NOT NULL,
    score double precision DEFAULT 0 NOT NULL,
    started_reading_date bigint DEFAULT 0 NOT NULL,
    finished_reading_date bigint DEFAULT 0 NOT NULL,
    private boolean DEFAULT false NOT NULL,
    authors character varying(256) DEFAULT NULL::character varying,
    artists character varying(256) DEFAULT NULL::character varying
);


--
-- Name: tracksearch_id_seq; Type: SEQUENCE; Schema: suwayomi; Owner: -
--

CREATE SEQUENCE suwayomi.tracksearch_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: tracksearch_id_seq; Type: SEQUENCE OWNED BY; Schema: suwayomi; Owner: -
--

ALTER SEQUENCE suwayomi.tracksearch_id_seq OWNED BY suwayomi.tracksearch.id;


--
-- Name: useraccount; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.useraccount (
    id integer NOT NULL,
    username character varying(64) NOT NULL,
    password character varying(90) NOT NULL,
    session_version integer DEFAULT 0 NOT NULL
);


--
-- Name: useraccount_id_seq; Type: SEQUENCE; Schema: suwayomi; Owner: -
--

CREATE SEQUENCE suwayomi.useraccount_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: useraccount_id_seq; Type: SEQUENCE OWNED BY; Schema: suwayomi; Owner: -
--

ALTER SEQUENCE suwayomi.useraccount_id_seq OWNED BY suwayomi.useraccount.id;


--
-- Name: usercode; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.usercode (
    id integer NOT NULL,
    user_id integer,
    type character varying(32) NOT NULL,
    code_hash character varying(90) NOT NULL,
    created_by integer NOT NULL,
    created_at bigint DEFAULT 0 NOT NULL,
    expires_at bigint DEFAULT 0 NOT NULL,
    consumed_at bigint,
    has_permissions boolean DEFAULT false NOT NULL
);


--
-- Name: usercode_id_seq; Type: SEQUENCE; Schema: suwayomi; Owner: -
--

CREATE SEQUENCE suwayomi.usercode_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: usercode_id_seq; Type: SEQUENCE OWNED BY; Schema: suwayomi; Owner: -
--

ALTER SEQUENCE suwayomi.usercode_id_seq OWNED BY suwayomi.usercode.id;


--
-- Name: usercodepermissions; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.usercodepermissions (
    user_code_id integer NOT NULL,
    permission character varying(128) NOT NULL
);


--
-- Name: userpermissions; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.userpermissions (
    user_id integer NOT NULL,
    permission character varying(128) NOT NULL
);


--
-- Name: userroles; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.userroles (
    user_id integer NOT NULL,
    role character varying(24) NOT NULL
);


--
-- Name: usersettings; Type: TABLE; Schema: suwayomi; Owner: -
--

CREATE TABLE suwayomi.usersettings (
    user_id integer NOT NULL,
    key character varying(256) NOT NULL,
    value character varying(16384) NOT NULL
);


--
-- Name: category id; Type: DEFAULT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.category ALTER COLUMN id SET DEFAULT nextval('suwayomi.category_id_seq'::regclass);


--
-- Name: categorymanga id; Type: DEFAULT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.categorymanga ALTER COLUMN id SET DEFAULT nextval('suwayomi.categorymanga_id_seq'::regclass);


--
-- Name: categorymeta id; Type: DEFAULT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.categorymeta ALTER COLUMN id SET DEFAULT nextval('suwayomi.categorymeta_id_seq'::regclass);


--
-- Name: chapter id; Type: DEFAULT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.chapter ALTER COLUMN id SET DEFAULT nextval('suwayomi.chapter_id_seq'::regclass);


--
-- Name: chaptermeta id; Type: DEFAULT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.chaptermeta ALTER COLUMN id SET DEFAULT nextval('suwayomi.chaptermeta_id_seq'::regclass);


--
-- Name: chapteruser id; Type: DEFAULT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.chapteruser ALTER COLUMN id SET DEFAULT nextval('suwayomi.chapteruser_id_seq'::regclass);


--
-- Name: extension id; Type: DEFAULT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.extension ALTER COLUMN id SET DEFAULT nextval('suwayomi.extension_id_seq'::regclass);


--
-- Name: extensionstore id; Type: DEFAULT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.extensionstore ALTER COLUMN id SET DEFAULT nextval('suwayomi.extensionstore_id_seq'::regclass);


--
-- Name: globalmeta id; Type: DEFAULT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.globalmeta ALTER COLUMN id SET DEFAULT nextval('suwayomi.globalmeta_id_seq'::regclass);


--
-- Name: manga id; Type: DEFAULT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.manga ALTER COLUMN id SET DEFAULT nextval('suwayomi.manga_id_seq'::regclass);


--
-- Name: mangameta id; Type: DEFAULT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.mangameta ALTER COLUMN id SET DEFAULT nextval('suwayomi.mangameta_id_seq'::regclass);


--
-- Name: mangauser id; Type: DEFAULT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.mangauser ALTER COLUMN id SET DEFAULT nextval('suwayomi.mangauser_id_seq'::regclass);


--
-- Name: page id; Type: DEFAULT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.page ALTER COLUMN id SET DEFAULT nextval('suwayomi.page_id_seq'::regclass);


--
-- Name: sourcemeta id; Type: DEFAULT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.sourcemeta ALTER COLUMN id SET DEFAULT nextval('suwayomi.sourcemeta_id_seq'::regclass);


--
-- Name: trackrecord id; Type: DEFAULT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.trackrecord ALTER COLUMN id SET DEFAULT nextval('suwayomi.trackrecord_id_seq'::regclass);


--
-- Name: tracksearch id; Type: DEFAULT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.tracksearch ALTER COLUMN id SET DEFAULT nextval('suwayomi.tracksearch_id_seq'::regclass);


--
-- Name: useraccount id; Type: DEFAULT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.useraccount ALTER COLUMN id SET DEFAULT nextval('suwayomi.useraccount_id_seq'::regclass);


--
-- Name: usercode id; Type: DEFAULT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.usercode ALTER COLUMN id SET DEFAULT nextval('suwayomi.usercode_id_seq'::regclass);


--
-- Name: category category_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.category
    ADD CONSTRAINT category_pkey PRIMARY KEY (id);


--
-- Name: categorymanga categorymanga_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.categorymanga
    ADD CONSTRAINT categorymanga_pkey PRIMARY KEY (id);


--
-- Name: categorymeta categorymeta_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.categorymeta
    ADD CONSTRAINT categorymeta_pkey PRIMARY KEY (id);


--
-- Name: chapter chapter_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.chapter
    ADD CONSTRAINT chapter_pkey PRIMARY KEY (id);


--
-- Name: chaptermeta chaptermeta_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.chaptermeta
    ADD CONSTRAINT chaptermeta_pkey PRIMARY KEY (id);


--
-- Name: chapteruser chapteruser_chapter_user_id_unique; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.chapteruser
    ADD CONSTRAINT chapteruser_chapter_user_id_unique UNIQUE (chapter, user_id);


--
-- Name: chapteruser chapteruser_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.chapteruser
    ADD CONSTRAINT chapteruser_pkey PRIMARY KEY (id);


--
-- Name: extension extension_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.extension
    ADD CONSTRAINT extension_pkey PRIMARY KEY (id);


--
-- Name: extensionstore extensionstore_index_url_unique; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.extensionstore
    ADD CONSTRAINT extensionstore_index_url_unique UNIQUE (index_url);


--
-- Name: extensionstore extensionstore_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.extensionstore
    ADD CONSTRAINT extensionstore_pkey PRIMARY KEY (id);


--
-- Name: globalmeta globalmeta_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.globalmeta
    ADD CONSTRAINT globalmeta_pkey PRIMARY KEY (id);


--
-- Name: manga manga_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.manga
    ADD CONSTRAINT manga_pkey PRIMARY KEY (id);


--
-- Name: mangameta mangameta_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.mangameta
    ADD CONSTRAINT mangameta_pkey PRIMARY KEY (id);


--
-- Name: mangauser mangauser_manga_user_id_unique; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.mangauser
    ADD CONSTRAINT mangauser_manga_user_id_unique UNIQUE (manga, user_id);


--
-- Name: mangauser mangauser_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.mangauser
    ADD CONSTRAINT mangauser_pkey PRIMARY KEY (id);


--
-- Name: migrations migrations_name_unique; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.migrations
    ADD CONSTRAINT migrations_name_unique UNIQUE (name);


--
-- Name: migrations migrations_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.migrations
    ADD CONSTRAINT migrations_pkey PRIMARY KEY (version);


--
-- Name: page page_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.page
    ADD CONSTRAINT page_pkey PRIMARY KEY (id);


--
-- Name: sourcemeta sourcemeta_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.sourcemeta
    ADD CONSTRAINT sourcemeta_pkey PRIMARY KEY (id);


--
-- Name: trackrecord trackrecord_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.trackrecord
    ADD CONSTRAINT trackrecord_pkey PRIMARY KEY (id);


--
-- Name: tracksearch tracksearch_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.tracksearch
    ADD CONSTRAINT tracksearch_pkey PRIMARY KEY (id);


--
-- Name: categorymanga uc_categorymanga_unique; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.categorymanga
    ADD CONSTRAINT uc_categorymanga_unique UNIQUE (user_id, category, manga);


--
-- Name: categorymeta uc_categorymeta_unique; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.categorymeta
    ADD CONSTRAINT uc_categorymeta_unique UNIQUE (user_id, category_ref, meta_key);


--
-- Name: chapter uc_chapter; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.chapter
    ADD CONSTRAINT uc_chapter UNIQUE (url, manga);


--
-- Name: chaptermeta uc_chaptermeta_unique; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.chaptermeta
    ADD CONSTRAINT uc_chaptermeta_unique UNIQUE (user_id, chapter_ref, meta_key);


--
-- Name: globalmeta uc_globalmeta_unique; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.globalmeta
    ADD CONSTRAINT uc_globalmeta_unique UNIQUE (user_id, meta_key);


--
-- Name: mangameta uc_mangameta_unique; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.mangameta
    ADD CONSTRAINT uc_mangameta_unique UNIQUE (user_id, manga_ref, meta_key);


--
-- Name: page uc_page; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.page
    ADD CONSTRAINT uc_page UNIQUE (index, chapter);


--
-- Name: extension uc_pkg_name; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.extension
    ADD CONSTRAINT uc_pkg_name UNIQUE (pkg_name);


--
-- Name: source uc_source_id; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.source
    ADD CONSTRAINT uc_source_id UNIQUE (id);


--
-- Name: sourcemeta uc_sourcemeta_unique; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.sourcemeta
    ADD CONSTRAINT uc_sourcemeta_unique UNIQUE (user_id, source_ref, meta_key);


--
-- Name: trackrecord uc_trackrecord_unique; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.trackrecord
    ADD CONSTRAINT uc_trackrecord_unique UNIQUE (user_id, manga_id, sync_id);


--
-- Name: tracksearch uc_tracksearch_tracker_id_remote_id; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.tracksearch
    ADD CONSTRAINT uc_tracksearch_tracker_id_remote_id UNIQUE (tracker_id, remote_id);


--
-- Name: useraccount useraccount_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.useraccount
    ADD CONSTRAINT useraccount_pkey PRIMARY KEY (id);


--
-- Name: useraccount useraccount_username_unique; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.useraccount
    ADD CONSTRAINT useraccount_username_unique UNIQUE (username);


--
-- Name: usercode usercode_pkey; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.usercode
    ADD CONSTRAINT usercode_pkey PRIMARY KEY (id);


--
-- Name: usercodepermissions usercodepermissions_user_code_id_permission_unique; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.usercodepermissions
    ADD CONSTRAINT usercodepermissions_user_code_id_permission_unique UNIQUE (user_code_id, permission);


--
-- Name: userpermissions userpermissions_user_id_permission_unique; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.userpermissions
    ADD CONSTRAINT userpermissions_user_id_permission_unique UNIQUE (user_id, permission);


--
-- Name: userroles userroles_user_id_role_unique; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.userroles
    ADD CONSTRAINT userroles_user_id_role_unique UNIQUE (user_id, role);


--
-- Name: usersettings usersettings_user_id_key_unique; Type: CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.usersettings
    ADD CONSTRAINT usersettings_user_id_key_unique UNIQUE (user_id, key);


--
-- Name: chapter_fetched_at; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX chapter_fetched_at ON suwayomi.chapter USING btree (fetched_at DESC, source_order DESC, id);


--
-- Name: chapter_manga_date_upload; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX chapter_manga_date_upload ON suwayomi.chapter USING btree (manga, date_upload DESC, source_order DESC);


--
-- Name: chapter_manga_fetched_at; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX chapter_manga_fetched_at ON suwayomi.chapter USING btree (manga, fetched_at DESC, source_order DESC);


--
-- Name: chapter_manga_source_order; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX chapter_manga_source_order ON suwayomi.chapter USING btree (manga, source_order);


--
-- Name: chapteruser_user_bookmarked_chapter; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX chapteruser_user_bookmarked_chapter ON suwayomi.chapteruser USING btree (user_id, bookmark, chapter);


--
-- Name: chapteruser_user_downloaded_chapter; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX chapteruser_user_downloaded_chapter ON suwayomi.chapteruser USING btree (user_id, is_downloaded, chapter);


--
-- Name: chapteruser_user_last_read_at_chapter; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX chapteruser_user_last_read_at_chapter ON suwayomi.chapteruser USING btree (user_id, last_read_at DESC, chapter);


--
-- Name: chapteruser_user_read_chapter; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX chapteruser_user_read_chapter ON suwayomi.chapteruser USING btree (user_id, read, chapter);


--
-- Name: extension_store_index_url; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX extension_store_index_url ON suwayomi.extension USING btree (store_index_url);


--
-- Name: idx_category_id_user_id; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX idx_category_id_user_id ON suwayomi.category USING btree (id, user_id);


--
-- Name: idx_category_user_id; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX idx_category_user_id ON suwayomi.category USING btree (user_id);


--
-- Name: idx_categorymanga_category; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX idx_categorymanga_category ON suwayomi.categorymanga USING btree (category);


--
-- Name: idx_categorymanga_manga; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX idx_categorymanga_manga ON suwayomi.categorymanga USING btree (manga);


--
-- Name: idx_categorymeta_category; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX idx_categorymeta_category ON suwayomi.categorymeta USING btree (category_ref);


--
-- Name: idx_chapter_manga; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX idx_chapter_manga ON suwayomi.chapter USING btree (manga);


--
-- Name: idx_chaptermeta_chapter; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX idx_chaptermeta_chapter ON suwayomi.chaptermeta USING btree (chapter_ref);


--
-- Name: idx_mangameta_manga; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX idx_mangameta_manga ON suwayomi.mangameta USING btree (manga_ref);


--
-- Name: idx_page_chapter; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX idx_page_chapter ON suwayomi.page USING btree (chapter);


--
-- Name: idx_source_extension; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX idx_source_extension ON suwayomi.source USING btree (extension);


--
-- Name: idx_sourcemeta_source; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX idx_sourcemeta_source ON suwayomi.sourcemeta USING btree (source_ref);


--
-- Name: idx_trackrecord_id_user_id; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX idx_trackrecord_id_user_id ON suwayomi.trackrecord USING btree (id, user_id);


--
-- Name: idx_trackrecord_manga; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX idx_trackrecord_manga ON suwayomi.trackrecord USING btree (manga_id);


--
-- Name: usercode_user_id_type_consumed_at; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE INDEX usercode_user_id_type_consumed_at ON suwayomi.usercode USING btree (user_id, type, consumed_at);


--
-- Name: ux_category_default_per_user; Type: INDEX; Schema: suwayomi; Owner: -
--

CREATE UNIQUE INDEX ux_category_default_per_user ON suwayomi.category USING btree (default_user_id);


--
-- Name: categorymanga delete_manga_category_update_version; Type: TRIGGER; Schema: suwayomi; Owner: -
--

CREATE TRIGGER delete_manga_category_update_version AFTER DELETE ON suwayomi.categorymanga FOR EACH ROW EXECUTE FUNCTION suwayomi.delete_manga_category_update_version();


--
-- Name: category insert_category_uid; Type: TRIGGER; Schema: suwayomi; Owner: -
--

CREATE TRIGGER insert_category_uid BEFORE INSERT ON suwayomi.category FOR EACH ROW EXECUTE FUNCTION suwayomi.insert_category_uid();


--
-- Name: categorymanga insert_manga_category_update_version; Type: TRIGGER; Schema: suwayomi; Owner: -
--

CREATE TRIGGER insert_manga_category_update_version AFTER INSERT ON suwayomi.categorymanga FOR EACH ROW EXECUTE FUNCTION suwayomi.insert_manga_category_update_version();


--
-- Name: trackrecord trackrecord_update_manga_version; Type: TRIGGER; Schema: suwayomi; Owner: -
--

CREATE TRIGGER trackrecord_update_manga_version AFTER INSERT OR DELETE OR UPDATE ON suwayomi.trackrecord FOR EACH ROW EXECUTE FUNCTION suwayomi.trackrecord_update_manga_version();


--
-- Name: category update_category_version; Type: TRIGGER; Schema: suwayomi; Owner: -
--

CREATE TRIGGER update_category_version BEFORE UPDATE ON suwayomi.category FOR EACH ROW EXECUTE FUNCTION suwayomi.update_category_version();


--
-- Name: chapteruser update_chapter_user_last_modified_at; Type: TRIGGER; Schema: suwayomi; Owner: -
--

CREATE TRIGGER update_chapter_user_last_modified_at BEFORE INSERT OR UPDATE ON suwayomi.chapteruser FOR EACH ROW EXECUTE FUNCTION suwayomi.update_chapter_user_last_modified_at();


--
-- Name: chapteruser update_chapter_user_version; Type: TRIGGER; Schema: suwayomi; Owner: -
--

CREATE TRIGGER update_chapter_user_version BEFORE UPDATE ON suwayomi.chapteruser FOR EACH ROW EXECUTE FUNCTION suwayomi.update_chapter_user_version();


--
-- Name: manga update_manga_bump_user_versions; Type: TRIGGER; Schema: suwayomi; Owner: -
--

CREATE TRIGGER update_manga_bump_user_versions AFTER UPDATE ON suwayomi.manga FOR EACH ROW EXECUTE FUNCTION suwayomi.update_manga_bump_user_versions();


--
-- Name: mangauser update_manga_user_last_modified_at; Type: TRIGGER; Schema: suwayomi; Owner: -
--

CREATE TRIGGER update_manga_user_last_modified_at BEFORE INSERT OR UPDATE ON suwayomi.mangauser FOR EACH ROW EXECUTE FUNCTION suwayomi.update_manga_user_last_modified_at();


--
-- Name: mangauser update_manga_user_version; Type: TRIGGER; Schema: suwayomi; Owner: -
--

CREATE TRIGGER update_manga_user_version BEFORE UPDATE ON suwayomi.mangauser FOR EACH ROW EXECUTE FUNCTION suwayomi.update_manga_user_version();


--
-- Name: categorymeta categorymeta_category_ref_fkey; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.categorymeta
    ADD CONSTRAINT categorymeta_category_ref_fkey FOREIGN KEY (category_ref) REFERENCES suwayomi.category(id) ON DELETE CASCADE;


--
-- Name: category fk_category_user_id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.category
    ADD CONSTRAINT fk_category_user_id FOREIGN KEY (user_id) REFERENCES suwayomi.useraccount(id) ON DELETE CASCADE;


--
-- Name: categorymanga fk_categorymanga_category__id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.categorymanga
    ADD CONSTRAINT fk_categorymanga_category__id FOREIGN KEY (category) REFERENCES suwayomi.category(id) ON DELETE CASCADE;


--
-- Name: categorymanga fk_categorymanga_manga__id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.categorymanga
    ADD CONSTRAINT fk_categorymanga_manga__id FOREIGN KEY (manga) REFERENCES suwayomi.manga(id) ON DELETE CASCADE;


--
-- Name: categorymanga fk_categorymanga_user_id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.categorymanga
    ADD CONSTRAINT fk_categorymanga_user_id FOREIGN KEY (user_id) REFERENCES suwayomi.useraccount(id) ON DELETE CASCADE;


--
-- Name: categorymeta fk_categorymeta_user_id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.categorymeta
    ADD CONSTRAINT fk_categorymeta_user_id FOREIGN KEY (user_id) REFERENCES suwayomi.useraccount(id) ON DELETE CASCADE;


--
-- Name: chapter fk_chapter_manga__id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.chapter
    ADD CONSTRAINT fk_chapter_manga__id FOREIGN KEY (manga) REFERENCES suwayomi.manga(id) ON DELETE CASCADE;


--
-- Name: chaptermeta fk_chaptermeta_chapter_ref__id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.chaptermeta
    ADD CONSTRAINT fk_chaptermeta_chapter_ref__id FOREIGN KEY (chapter_ref) REFERENCES suwayomi.chapter(id) ON UPDATE RESTRICT ON DELETE CASCADE;


--
-- Name: chaptermeta fk_chaptermeta_user_id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.chaptermeta
    ADD CONSTRAINT fk_chaptermeta_user_id FOREIGN KEY (user_id) REFERENCES suwayomi.useraccount(id) ON DELETE CASCADE;


--
-- Name: chapteruser fk_chapteruser_chapter__id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.chapteruser
    ADD CONSTRAINT fk_chapteruser_chapter__id FOREIGN KEY (chapter) REFERENCES suwayomi.chapter(id) ON UPDATE RESTRICT ON DELETE CASCADE;


--
-- Name: chapteruser fk_chapteruser_user_id__id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.chapteruser
    ADD CONSTRAINT fk_chapteruser_user_id__id FOREIGN KEY (user_id) REFERENCES suwayomi.useraccount(id) ON UPDATE RESTRICT ON DELETE CASCADE;


--
-- Name: globalmeta fk_globalmeta_user_id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.globalmeta
    ADD CONSTRAINT fk_globalmeta_user_id FOREIGN KEY (user_id) REFERENCES suwayomi.useraccount(id) ON DELETE CASCADE;


--
-- Name: mangameta fk_mangameta_manga_ref__id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.mangameta
    ADD CONSTRAINT fk_mangameta_manga_ref__id FOREIGN KEY (manga_ref) REFERENCES suwayomi.manga(id) ON UPDATE RESTRICT ON DELETE CASCADE;


--
-- Name: mangameta fk_mangameta_user_id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.mangameta
    ADD CONSTRAINT fk_mangameta_user_id FOREIGN KEY (user_id) REFERENCES suwayomi.useraccount(id) ON DELETE CASCADE;


--
-- Name: mangauser fk_mangauser_manga__id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.mangauser
    ADD CONSTRAINT fk_mangauser_manga__id FOREIGN KEY (manga) REFERENCES suwayomi.manga(id) ON UPDATE RESTRICT ON DELETE CASCADE;


--
-- Name: mangauser fk_mangauser_user_id__id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.mangauser
    ADD CONSTRAINT fk_mangauser_user_id__id FOREIGN KEY (user_id) REFERENCES suwayomi.useraccount(id) ON UPDATE RESTRICT ON DELETE CASCADE;


--
-- Name: page fk_page_chapter__id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.page
    ADD CONSTRAINT fk_page_chapter__id FOREIGN KEY (chapter) REFERENCES suwayomi.chapter(id) ON DELETE CASCADE;


--
-- Name: source fk_source_extension__id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.source
    ADD CONSTRAINT fk_source_extension__id FOREIGN KEY (extension) REFERENCES suwayomi.extension(id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: sourcemeta fk_sourcemeta_user_id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.sourcemeta
    ADD CONSTRAINT fk_sourcemeta_user_id FOREIGN KEY (user_id) REFERENCES suwayomi.useraccount(id) ON DELETE CASCADE;


--
-- Name: trackrecord fk_trackrecord_manga_id__id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.trackrecord
    ADD CONSTRAINT fk_trackrecord_manga_id__id FOREIGN KEY (manga_id) REFERENCES suwayomi.manga(id) ON UPDATE RESTRICT ON DELETE CASCADE;


--
-- Name: trackrecord fk_trackrecord_user_id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.trackrecord
    ADD CONSTRAINT fk_trackrecord_user_id FOREIGN KEY (user_id) REFERENCES suwayomi.useraccount(id) ON DELETE CASCADE;


--
-- Name: usercode fk_usercode_created_by__id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.usercode
    ADD CONSTRAINT fk_usercode_created_by__id FOREIGN KEY (created_by) REFERENCES suwayomi.useraccount(id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: usercode fk_usercode_user_id__id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.usercode
    ADD CONSTRAINT fk_usercode_user_id__id FOREIGN KEY (user_id) REFERENCES suwayomi.useraccount(id) ON UPDATE RESTRICT ON DELETE CASCADE;


--
-- Name: usercodepermissions fk_usercodepermissions_user_code_id__id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.usercodepermissions
    ADD CONSTRAINT fk_usercodepermissions_user_code_id__id FOREIGN KEY (user_code_id) REFERENCES suwayomi.usercode(id) ON UPDATE RESTRICT ON DELETE CASCADE;


--
-- Name: userpermissions fk_userpermissions_user_id__id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.userpermissions
    ADD CONSTRAINT fk_userpermissions_user_id__id FOREIGN KEY (user_id) REFERENCES suwayomi.useraccount(id) ON UPDATE RESTRICT ON DELETE CASCADE;


--
-- Name: userroles fk_userroles_user_id__id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.userroles
    ADD CONSTRAINT fk_userroles_user_id__id FOREIGN KEY (user_id) REFERENCES suwayomi.useraccount(id) ON UPDATE RESTRICT ON DELETE CASCADE;


--
-- Name: usersettings fk_usersettings_user_id__id; Type: FK CONSTRAINT; Schema: suwayomi; Owner: -
--

ALTER TABLE ONLY suwayomi.usersettings
    ADD CONSTRAINT fk_usersettings_user_id__id FOREIGN KEY (user_id) REFERENCES suwayomi.useraccount(id) ON UPDATE RESTRICT ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

