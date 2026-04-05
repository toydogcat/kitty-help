--
-- PostgreSQL database dump
--

\restrict t47tirggzjyoKuXR93h8vCcHSUSWtWEAXvMLKRv5MekxnX2nWZofyL0jfG22lIC

-- Dumped from database version 15.17 (Debian 15.17-1.pgdg12+1)
-- Dumped by pg_dump version 15.17 (Debian 15.17-1.pgdg12+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: vector; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;


--
-- Name: EXTENSION vector; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION vector IS 'vector data type and ivfflat and hnsw access methods';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: bookmarks; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.bookmarks (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid,
    title text NOT NULL,
    url text NOT NULL,
    category text DEFAULT 'uncategorized'::text,
    icon_url text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    password_id uuid
);


ALTER TABLE public.bookmarks OWNER TO postgres;

--
-- Name: bot_auth_requests; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.bot_auth_requests (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    token text,
    platform text NOT NULL,
    account_id text NOT NULL,
    account_name text,
    status text DEFAULT 'pending'::text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.bot_auth_requests OWNER TO postgres;

--
-- Name: bot_authorized_users; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.bot_authorized_users (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    platform text NOT NULL,
    account_id text NOT NULL,
    account_name text,
    role text DEFAULT 'user'::text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.bot_authorized_users OWNER TO postgres;

--
-- Name: bulletin; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.bulletin (
    id integer NOT NULL,
    message text,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.bulletin OWNER TO postgres;

--
-- Name: bulletin_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.bulletin_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.bulletin_id_seq OWNER TO postgres;

--
-- Name: bulletin_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.bulletin_id_seq OWNED BY public.bulletin.id;


--
-- Name: calendar_events; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.calendar_events (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid,
    event_date text NOT NULL,
    content text NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.calendar_events OWNER TO postgres;

--
-- Name: common_state; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.common_state (
    key text NOT NULL,
    content text,
    file_url text,
    file_name text,
    updated_by uuid,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.common_state OWNER TO postgres;

--
-- Name: devices; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.devices (
    id text NOT NULL,
    status text DEFAULT 'pending'::text,
    device_name text,
    user_agent text,
    user_id uuid,
    last_active timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.devices OWNER TO postgres;

--
-- Name: impression_edges; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.impression_edges (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid,
    source_id uuid,
    target_id uuid,
    label text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.impression_edges OWNER TO postgres;

--
-- Name: impression_nodes; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.impression_nodes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid,
    media_id uuid,
    title text NOT NULL,
    content text,
    node_type text DEFAULT 'general'::text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.impression_nodes OWNER TO postgres;

--
-- Name: media_archives; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.media_archives (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    file_id text NOT NULL,
    message_id bigint,
    media_type text NOT NULL,
    caption text,
    source_platform text DEFAULT 'telegram'::text,
    sender_name text,
    sender_id text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    title text,
    notes text,
    embedding public.vector(1536),
    metadata jsonb,
    is_indexable boolean DEFAULT false,
    index_status text DEFAULT 'not_indexed'::text,
    embedding_model text
);


ALTER TABLE public.media_archives OWNER TO postgres;

--
-- Name: passwords; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.passwords (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid,
    site_name text NOT NULL,
    account text NOT NULL,
    password_raw text NOT NULL,
    category text,
    notes text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.passwords OWNER TO postgres;

--
-- Name: security_sessions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.security_sessions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid,
    device_id text NOT NULL,
    token text NOT NULL,
    line_verified_at timestamp without time zone,
    discord_verified_at timestamp without time zone,
    expires_at timestamp without time zone NOT NULL,
    granted_at timestamp without time zone,
    status text DEFAULT 'pending'::text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.security_sessions OWNER TO postgres;

--
-- Name: settings; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.settings (
    key text NOT NULL,
    value text NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.settings OWNER TO postgres;

--
-- Name: snippets; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.snippets (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid,
    parent_id uuid,
    name text NOT NULL,
    content text,
    is_folder boolean DEFAULT false,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.snippets OWNER TO postgres;

--
-- Name: storehouse_items; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.storehouse_items (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    telegram_msg_id bigint,
    file_id text,
    file_name text,
    file_size bigint,
    mime_type text,
    category text,
    description text,
    chat_id bigint,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.storehouse_items OWNER TO postgres;

--
-- Name: user_emails; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.user_emails (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid,
    email text NOT NULL,
    created_at timestamp without time zone DEFAULT now()
);


ALTER TABLE public.user_emails OWNER TO postgres;

--
-- Name: users; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.users (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    role text DEFAULT 'user'::text,
    google_id text,
    email text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    line_id text,
    discord_id text,
    nickname text,
    telegram_id text,
    main_email text
);


ALTER TABLE public.users OWNER TO postgres;

--
-- Name: bulletin id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.bulletin ALTER COLUMN id SET DEFAULT nextval('public.bulletin_id_seq'::regclass);


--
-- Data for Name: bookmarks; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.bookmarks (id, user_id, title, url, category, icon_url, created_at, password_id) FROM stdin;
\.


--
-- Data for Name: bot_auth_requests; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.bot_auth_requests (id, token, platform, account_id, account_name, status, created_at) FROM stdin;
\.


--
-- Data for Name: bot_authorized_users; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.bot_authorized_users (id, platform, account_id, account_name, role, created_at) FROM stdin;
686f97d5-a953-48b5-84dd-066c1c682e80	telegram	1089079202	Admin From Env	superadmin	2026-04-04 13:32:43.596775
b077a395-df39-44c8-8013-c0a40c86bf1e	discord	840468194456371211	Admin From Env	superadmin	2026-04-04 13:32:43.604067
482ccb90-8512-43b0-b6ff-33e0be50c57e	line	Uaecf740fc05ef668b671fa90da9c832e	Admin From Env	superadmin	2026-04-04 13:32:43.606193
\.


--
-- Data for Name: bulletin; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.bulletin (id, message, updated_at) FROM stdin;
1	Welcome back, Kitty-Admin!	2026-04-04 15:16:43.646953
2	我是超級好棒棒	2026-04-05 04:56:24.136394
\.


--
-- Data for Name: calendar_events; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.calendar_events (id, user_id, event_date, content, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: common_state; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.common_state (key, content, file_url, file_name, updated_by, updated_at) FROM stdin;
\.


--
-- Data for Name: devices; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.devices (id, status, device_name, user_agent, user_id, last_active, created_at) FROM stdin;
93ee7ef9-2055-438c-bf0f-ebf5aaeb0860	pending	\N	Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36	\N	2026-04-05 07:26:30.223057	2026-04-04 15:04:48.040271
\.


--
-- Data for Name: impression_edges; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.impression_edges (id, user_id, source_id, target_id, label, created_at) FROM stdin;
decef8b3-15d8-419c-af07-59056cbfc94a	41023b15-a1db-4aac-aab8-ba75d8d90905	6d72a8fe-09c9-4194-a5c7-a42d061d20ba	690b4694-878e-4e1e-83c9-25709d569e81	飼養	2026-04-05 04:27:34.594397
dbec2945-4cea-4fee-9023-21ae8f2d5cee	41023b15-a1db-4aac-aab8-ba75d8d90905	6d72a8fe-09c9-4194-a5c7-a42d061d20ba	460ef2f4-3924-40f5-8f0d-4c207f87321c	抱抱	2026-04-05 04:29:36.244151
7cb0e637-c03c-4ac3-84b6-1d7128cfc69d	41023b15-a1db-4aac-aab8-ba75d8d90905	6d72a8fe-09c9-4194-a5c7-a42d061d20ba	182627e6-6e7f-44d6-8525-2eed7f88395b	天空	2026-04-05 03:27:58.605173
f7474f17-7a7a-4e70-85c4-2c1924ff078a	41023b15-a1db-4aac-aab8-ba75d8d90905	690b4694-878e-4e1e-83c9-25709d569e81	8e425cfa-bcb9-40f9-b0c8-dbd0d3c28b9d	愛讀	2026-04-05 04:52:31.267483
\.


--
-- Data for Name: impression_nodes; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.impression_nodes (id, user_id, media_id, title, content, node_type, created_at) FROM stdin;
181242cc-7bd2-4600-ad1c-2aa409b0ac1a	41023b15-a1db-4aac-aab8-ba75d8d90905	3e16102f-bc32-4a2d-a4cd-9cac56bbf8ba	top	5345413	general	2026-04-05 02:59:42.258681
182627e6-6e7f-44d6-8525-2eed7f88395b	41023b15-a1db-4aac-aab8-ba75d8d90905	3e16102f-bc32-4a2d-a4cd-9cac56bbf8ba	Top	4534	person	2026-04-05 03:03:49.576429
690b4694-878e-4e1e-83c9-25709d569e81	41023b15-a1db-4aac-aab8-ba75d8d90905	b345badb-8238-4872-a367-b292cc6e7854	鼠鼠	8764	work	2026-04-05 04:24:51.439973
460ef2f4-3924-40f5-8f0d-4c207f87321c	41023b15-a1db-4aac-aab8-ba75d8d90905	644c9907-20bb-4140-b81b-ad188e5b48fa	貓狗	56465341	general	2026-04-05 04:25:21.783231
6d72a8fe-09c9-4194-a5c7-a42d061d20ba	41023b15-a1db-4aac-aab8-ba75d8d90905	cede26a7-3124-4cd0-a49f-b3e02b50819c	Lulu	46584	person	2026-04-05 03:09:15.65067
8e425cfa-bcb9-40f9-b0c8-dbd0d3c28b9d	41023b15-a1db-4aac-aab8-ba75d8d90905	1d80dda9-bb44-4f1f-a48a-084af520a633	Copilot	讀書會要用	work	2026-04-05 04:51:26.259387
\.


--
-- Data for Name: media_archives; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.media_archives (id, file_id, message_id, media_type, caption, source_platform, sender_name, sender_id, created_at, title, notes, embedding, metadata, is_indexable, index_status, embedding_model) FROM stdin;
5c53c1d2-e6c2-491f-acd2-dc9a6ace5f9d	rat_photo_0404	1488397212399439912	photo	📦 **Media Backup**\n**Source Platform**: `discord`\n**Sender ID**: `840468194456371211`\n**Chat ID**: `1488397212399439912`\n**Timestamp**: `2026-04-04 12:30:05`\n**Username**: @mouseking0445	discord	mouseking0445	840468194456371211	2026-04-04 12:30:05	\N	\N	\N	\N	f	not_indexed	\N
fc185e0a-6a0d-465d-b794-f1179a545ee3	AgACAgUAAxkBAAMNadEtEjE9D4S09rtM0_H0rWjDyzUAAtoOaxs904lW_2TlqcPtCrUBAAMCAAN4AAM7BA	13	photo		telegram	toydogcat wang	1089079202	2026-04-04 15:24:03.264374	貓貓與狗狗	初期測試	\N	\N	f	not_indexed	\N
de299c04-f9c6-445e-ac49-5b139af71c65	AgACAgUAAyEGAATenYvbAAMVadG-be6c5QXo0XiYj3yi1Zw4Zb0AAuwMaxvcOJFWN1jha4ibOkoBAAMCAAN5AAM7BA	0	photo		line	\N	Uaecf740fc05ef668b671fa90da9c832e	2026-04-05 01:44:13.795739	\N	\N	\N	\N	f	not_indexed	\N
3e16102f-bc32-4a2d-a4cd-9cac56bbf8ba	AgACAgUAAyEGAATenYvbAAMWadG_n_8E14MQAAF6D1AGUcbb-AY1AALvDGsb3DiRVh2hMfwuWpbpAQADAgADeQADOwQ	1490166402651525372	photo		discord	mouseking0445	840468194456371211	2026-04-05 01:49:19.955225	\N	\N	\N	\N	t	not_indexed	\N
913c3f81-b2e9-4b00-b627-c0fc05f61bce	AgACAgUAAyEGAATenYvbAAMXadG_-5gTxm7LMmdzteSeNcoquyEAAvAMaxvcOJFWlOSglfYusVEBAAMCAAN5AAM7BA	0	photo		line	玩具狗	Uaecf740fc05ef668b671fa90da9c832e	2026-04-05 01:50:51.647833	\N	\N	\N	\N	t	not_indexed	\N
b345badb-8238-4872-a367-b292cc6e7854	AgACAgUAAyEGAATenYvbAAMaadHb7LVYWr4AAWECYdQuynbI6SHlAAIoDWsb3DiRVmy4KErCwdokAQADAgADdwADOwQ	1490196788634976398	photo	鼠鼠	discord	mouseking0445	840468194456371211	2026-04-05 03:50:04.599126	\N	\N	\N	\N	t	not_indexed	\N
644c9907-20bb-4140-b81b-ad188e5b48fa	AgACAgUAAyEGAATenYvbAAMbadHcEL53HdncQ70T3xtfGfa9Q7gAAikNaxvcOJFWC0NzBIijF9cBAAMCAAN4AAM7BA	1490196945363275786	photo		discord	mouseking0445	840468194456371211	2026-04-05 03:50:40.448049	\N	\N	\N	\N	t	not_indexed	\N
1d80dda9-bb44-4f1f-a48a-084af520a633	AgACAgUAAyEGAATenYvbAAMdadHpAyQg6fk1-R9vDExew3-TUsgAAkgNaxvcOJFW_zWmDqXnHQIBAAMCAAN5AAM7BA	1490210830351073290	photo		discord	mouseking0445	840468194456371211	2026-04-05 04:45:55.286723	\N	\N	\N	\N	t	not_indexed	\N
cede26a7-3124-4cd0-a49f-b3e02b50819c	AgACAgUAAyEGAATenYvbAAMYadHSQSIJ04Qq6fMGTMehGdeJq2QAAhkNaxvcOJFWqdAWFweyfW0BAAMCAAN5AAM7BA	1490186402539769967	photo		discord	mouseking0445	840468194456371211	2026-04-05 03:08:49.506739	如如	如如在郵輪電梯	\N	\N	t	not_indexed	\N
\.


--
-- Data for Name: passwords; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.passwords (id, user_id, site_name, account, password_raw, category, notes, created_at) FROM stdin;
\.


--
-- Data for Name: security_sessions; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.security_sessions (id, user_id, device_id, token, line_verified_at, discord_verified_at, expires_at, granted_at, status, created_at) FROM stdin;
\.


--
-- Data for Name: settings; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.settings (key, value, updated_at) FROM stdin;
\.


--
-- Data for Name: snippets; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.snippets (id, user_id, parent_id, name, content, is_folder, created_at) FROM stdin;
\.


--
-- Data for Name: storehouse_items; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.storehouse_items (id, telegram_msg_id, file_id, file_name, file_size, mime_type, category, description, chat_id, created_at) FROM stdin;
\.


--
-- Data for Name: user_emails; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.user_emails (id, user_id, email, created_at) FROM stdin;
2237e035-508e-48ca-b2c1-5896a2938fff	41023b15-a1db-4aac-aab8-ba75d8d90905	tobywang2021@gmail.com	2026-04-05 02:22:06.987362
9da7c6d8-2d3a-4b8f-b07f-c4570441ae35	41023b15-a1db-4aac-aab8-ba75d8d90905	mousekingfat@gmail.com	2026-04-05 02:22:06.987362
1520123e-08df-4323-bf05-7922eef3350f	41023b15-a1db-4aac-aab8-ba75d8d90905	chickenmilktea@gmail.com	2026-04-05 02:22:06.987362
\.


--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: postgres
--

COPY public.users (id, name, role, google_id, email, created_at, line_id, discord_id, nickname, telegram_id, main_email) FROM stdin;
82507694-4205-49d4-8099-9e18ba997581	Master Admin	superadmin	\N	toby@family.local	2026-04-04 14:30:34.562527	\N	\N	\N	\N	toby@family.local
41023b15-a1db-4aac-aab8-ba75d8d90905	Toby-Admin	admin	\N	toydogcat@gmail.com	2026-04-05 02:22:06.987362	Uaecf740fc05ef668b671fa90da9c832e	840468194456371211	Toby	1089079202	toydogcat@gmail.com
\.


--
-- Name: bulletin_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

SELECT pg_catalog.setval('public.bulletin_id_seq', 2, true);


--
-- Name: bookmarks bookmarks_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.bookmarks
    ADD CONSTRAINT bookmarks_pkey PRIMARY KEY (id);


--
-- Name: bot_auth_requests bot_auth_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.bot_auth_requests
    ADD CONSTRAINT bot_auth_requests_pkey PRIMARY KEY (id);


--
-- Name: bot_auth_requests bot_auth_requests_token_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.bot_auth_requests
    ADD CONSTRAINT bot_auth_requests_token_key UNIQUE (token);


--
-- Name: bot_authorized_users bot_authorized_users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.bot_authorized_users
    ADD CONSTRAINT bot_authorized_users_pkey PRIMARY KEY (id);


--
-- Name: bot_authorized_users bot_authorized_users_platform_account_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.bot_authorized_users
    ADD CONSTRAINT bot_authorized_users_platform_account_id_key UNIQUE (platform, account_id);


--
-- Name: bulletin bulletin_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.bulletin
    ADD CONSTRAINT bulletin_pkey PRIMARY KEY (id);


--
-- Name: calendar_events calendar_events_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.calendar_events
    ADD CONSTRAINT calendar_events_pkey PRIMARY KEY (id);


--
-- Name: calendar_events calendar_events_user_id_event_date_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.calendar_events
    ADD CONSTRAINT calendar_events_user_id_event_date_key UNIQUE (user_id, event_date);


--
-- Name: common_state common_state_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.common_state
    ADD CONSTRAINT common_state_pkey PRIMARY KEY (key);


--
-- Name: devices devices_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.devices
    ADD CONSTRAINT devices_pkey PRIMARY KEY (id);


--
-- Name: impression_edges impression_edges_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.impression_edges
    ADD CONSTRAINT impression_edges_pkey PRIMARY KEY (id);


--
-- Name: impression_nodes impression_nodes_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.impression_nodes
    ADD CONSTRAINT impression_nodes_pkey PRIMARY KEY (id);


--
-- Name: media_archives media_archives_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.media_archives
    ADD CONSTRAINT media_archives_pkey PRIMARY KEY (id);


--
-- Name: passwords passwords_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.passwords
    ADD CONSTRAINT passwords_pkey PRIMARY KEY (id);


--
-- Name: security_sessions security_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.security_sessions
    ADD CONSTRAINT security_sessions_pkey PRIMARY KEY (id);


--
-- Name: settings settings_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.settings
    ADD CONSTRAINT settings_pkey PRIMARY KEY (key);


--
-- Name: snippets snippets_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.snippets
    ADD CONSTRAINT snippets_pkey PRIMARY KEY (id);


--
-- Name: storehouse_items storehouse_items_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.storehouse_items
    ADD CONSTRAINT storehouse_items_pkey PRIMARY KEY (id);


--
-- Name: storehouse_items storehouse_items_telegram_msg_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.storehouse_items
    ADD CONSTRAINT storehouse_items_telegram_msg_id_key UNIQUE (telegram_msg_id);


--
-- Name: users unique_discord_id; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT unique_discord_id UNIQUE (discord_id);


--
-- Name: users unique_line_id; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT unique_line_id UNIQUE (line_id);


--
-- Name: users unique_main_email; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT unique_main_email UNIQUE (main_email);


--
-- Name: users unique_telegram_id; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT unique_telegram_id UNIQUE (telegram_id);


--
-- Name: user_emails user_emails_email_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_emails
    ADD CONSTRAINT user_emails_email_key UNIQUE (email);


--
-- Name: user_emails user_emails_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_emails
    ADD CONSTRAINT user_emails_pkey PRIMARY KEY (id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_google_id_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_google_id_key UNIQUE (google_id);


--
-- Name: users users_name_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_name_key UNIQUE (name);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: bookmarks bookmarks_password_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.bookmarks
    ADD CONSTRAINT bookmarks_password_id_fkey FOREIGN KEY (password_id) REFERENCES public.passwords(id) ON DELETE SET NULL;


--
-- Name: bookmarks bookmarks_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.bookmarks
    ADD CONSTRAINT bookmarks_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: devices devices_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.devices
    ADD CONSTRAINT devices_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: impression_edges impression_edges_source_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.impression_edges
    ADD CONSTRAINT impression_edges_source_id_fkey FOREIGN KEY (source_id) REFERENCES public.impression_nodes(id) ON DELETE CASCADE;


--
-- Name: impression_edges impression_edges_target_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.impression_edges
    ADD CONSTRAINT impression_edges_target_id_fkey FOREIGN KEY (target_id) REFERENCES public.impression_nodes(id) ON DELETE CASCADE;


--
-- Name: impression_edges impression_edges_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.impression_edges
    ADD CONSTRAINT impression_edges_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: impression_nodes impression_nodes_media_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.impression_nodes
    ADD CONSTRAINT impression_nodes_media_id_fkey FOREIGN KEY (media_id) REFERENCES public.media_archives(id) ON DELETE SET NULL;


--
-- Name: impression_nodes impression_nodes_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.impression_nodes
    ADD CONSTRAINT impression_nodes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: passwords passwords_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.passwords
    ADD CONSTRAINT passwords_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: security_sessions security_sessions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.security_sessions
    ADD CONSTRAINT security_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: snippets snippets_parent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.snippets
    ADD CONSTRAINT snippets_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.snippets(id) ON DELETE CASCADE;


--
-- Name: snippets snippets_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.snippets
    ADD CONSTRAINT snippets_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_emails user_emails_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_emails
    ADD CONSTRAINT user_emails_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict t47tirggzjyoKuXR93h8vCcHSUSWtWEAXvMLKRv5MekxnX2nWZofyL0jfG22lIC

