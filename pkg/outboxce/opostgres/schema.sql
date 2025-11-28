--
-- PostgreSQL database dump
--

\restrict Toa8a6L7YNZvn15s6XKH59wmGcU4WFYfuaVWqfqn75aKKR0t8SU4iPrR6fjfMh6

-- Dumped from database version 16.11 (Debian 16.11-1.pgdg13+1)
-- Dumped by pg_dump version 16.11 (Debian 16.11-1.pgdg13+1)

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

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: outboxce; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.outboxce (
    id uuid NOT NULL,
    tenant_id uuid,
    cloud_event json NOT NULL,
    is_delivered boolean,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: outboxce outboxce_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.outboxce
    ADD CONSTRAINT outboxce_pkey PRIMARY KEY (id);


--
-- Name: outboxce_by_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX outboxce_by_created_at ON public.outboxce USING btree (created_at) WHERE (is_delivered = false);


--
-- PostgreSQL database dump complete
--

\unrestrict Toa8a6L7YNZvn15s6XKH59wmGcU4WFYfuaVWqfqn75aKKR0t8SU4iPrR6fjfMh6

